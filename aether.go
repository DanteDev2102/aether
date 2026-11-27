package aether

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// DefaultMaxBodySize is the default maximum body size for incoming requests.
const DefaultMaxBodySize = 2 << 20 // 2MB

// DefaultShutdownTimeout is how long Listen waits for in-flight requests to
// finish once its context is canceled, unless Config.ShutdownTimeout is set.
const DefaultShutdownTimeout = 10 * time.Second

// TemplateEngine defines the interface for rendering templates.
type TemplateEngine interface {
	Render(w io.Writer, name string, data any) error
}

// Config holds Aether configuration options.
type Config[T any] struct {
	Port int
	Host string
	// Timeout bounds each request: it sets the request context deadline and
	// the server read and write timeouts. Zero means no timeout.
	Timeout time.Duration
	// ShutdownTimeout is how long graceful shutdown waits for in-flight
	// requests. Defaults to DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
	// TLSConfig is used by ListenTLS and ServeTLS. Optional.
	TLSConfig    *tls.Config
	JSON         JSONEngine
	XML          XMLEngine
	Template     TemplateEngine
	Cache        CacheStore
	Logger       Logger
	Global       T
	ErrorHandler CustomErrorHandler[T]
}

// App represents the main Aether application instance.
type App[T any] struct {
	frozen atomic.Bool
	config *Config[T]
	router *Router[T]
	cron   *CronManager
}

// New creates a new Aether application instance.
func New[T any](conf *Config[T]) *App[T] {
	if conf.JSON == nil {
		conf.JSON = stdJSONEngine{}
	}
	if conf.XML == nil {
		conf.XML = stdXMLEngine{}
	}
	if conf.Logger == nil {
		conf.Logger = newStdLogger()
	}
	if conf.ShutdownTimeout == 0 {
		conf.ShutdownTimeout = DefaultShutdownTimeout
	}
	if conf.MaxBodyBytes == 0 {
		conf.MaxBodyBytes = DefaultMaxBodySize
	}
	router := newRouter[T](conf)
	router.Use(RecoveryMiddleware[T](conf.ErrorHandler))

	return &App[T]{
		config: conf,
		router: router,
		cron:   newCronManager(conf.Logger),
	}
}

// Router returns the router instance.
func (a *App[T]) Router() *Router[T] {
	return a.router
}

// AddCron registers a new cron job with the specified name and interval.
// Jobs must be added before the app starts serving.
func (a *App[T]) AddCron(name string, interval time.Duration, job CronJob) {
	if a.frozen.Load() {
		a.config.Logger.Error("Cannot add cronjobs after Aether is listening")
		return
	}
	a.cron.Add(name, interval, job)
}

// Addr returns the address built from Config.Host and Config.Port.
func (a *App[T]) Addr() string {
	return net.JoinHostPort(a.config.Host, fmt.Sprint(a.config.Port))
}

// Listen binds Config.Host:Config.Port and serves HTTP until ctx is canceled,
// then shuts down gracefully. It returns an error if the address cannot be
// bound or the server fails; a graceful shutdown returns nil.
//
// To stop on SIGINT/SIGTERM, pass a context from signal.NotifyContext.
func (a *App[T]) Listen(ctx context.Context) error {
	ln, err := a.listen(ctx)
	if err != nil {
		return err
	}
	return a.Serve(ctx, ln)
}

// ListenTLS is like Listen but serves HTTPS using the given certificate and
// key files. Config.TLSConfig is used when set.
func (a *App[T]) ListenTLS(ctx context.Context, certFile, keyFile string) error {
	ln, err := a.listen(ctx)
	if err != nil {
		return err
	}
	return a.ServeTLS(ctx, ln, certFile, keyFile)
}

// Serve serves HTTP on an existing listener until ctx is canceled. It is
// useful for tests or socket activation; the listener is closed on return.
func (a *App[T]) Serve(ctx context.Context, ln net.Listener) error {
	srv := a.newServer()
	return a.run(ctx, srv, ln.Addr().String(), func() error { return srv.Serve(ln) })
}

// ServeTLS is like Serve but serves HTTPS using the given certificate and key
// files. Empty paths are allowed when Config.TLSConfig provides certificates.
func (a *App[T]) ServeTLS(ctx context.Context, ln net.Listener, certFile, keyFile string) error {
	srv := a.newServer()
	return a.run(ctx, srv, ln.Addr().String(), func() error { return srv.ServeTLS(ln, certFile, keyFile) })
}

func (a *App[T]) listen(ctx context.Context) (net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", a.Addr())
	if err != nil {
		return nil, fmt.Errorf("aether: listen on %s: %w", a.Addr(), err)
	}
	return ln, nil
}

func (a *App[T]) newServer() *http.Server {
	srv := &http.Server{
		Handler:           a.router,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         a.config.TLSConfig,
	}
	if t := a.config.Timeout; t > 0 {
		srv.ReadTimeout = t
		srv.WriteTimeout = t
		srv.IdleTimeout = t * 2
	}
	return srv
}

// run starts the cron jobs and the server, waits for ctx to be canceled or the
// server to fail, then shuts everything down.
func (a *App[T]) run(ctx context.Context, srv *http.Server, addr string, serve func() error) error {
	if !a.frozen.CompareAndSwap(false, true) {
		return errors.New("aether: app is already serving")
	}

	log := a.config.Logger
	a.cron.Start()
	log.Infof("Aether is up and flying! %s", addr)

	errCh := make(chan error, 1)
	go func() { errCh <- serve() }()

	var serveErr error
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("aether: server failed: %w", err)
		}
	case <-ctx.Done():
		log.Warn("Aether is gently shutting down... Waiting for connections to finish.")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.config.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("Aether forced to shutdown: %v", err)
		serveErr = errors.Join(serveErr, fmt.Errorf("aether: shutdown: %w", err))
	}

	a.cron.Stop()

	log.Info("Aether has successfully shutdown. See you soon!")
	log.Sync()

	return serveErr
}
