package aether

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DefaultMaxBodySize is the default maximum body size for incoming requests.
const DefaultMaxBodySize = 2 << 20 // 2MB

// TemplateEngine defines the interface for rendering templates.
type TemplateEngine interface {
	Render(w io.Writer, name string, data any) error
}

// Config holds Aether configuration options.
type Config[T any] struct {
	Port            int
	Host            string
	Timeout         int
	ShutdownTimeout int
	MaxBodyBytes    int64
	JSON            JSONEngine
	XML             XMLEngine
	Template        TemplateEngine
	Cache           CacheStore
	Logger          Logger
	Global          T
	ErrorHandler    CustomErrorHandler[T]
}

// App represents the main Aether application instance.
type App[T any] struct {
	frozen bool
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
		conf.ShutdownTimeout = 10
	}
	if conf.MaxBodyBytes == 0 {
		conf.MaxBodyBytes = DefaultMaxBodySize
	}
	router := NewRouter[T](conf.JSON, conf.XML, conf.Template, conf.Cache, conf.Logger, conf.Global, conf.Timeout, conf.MaxBodyBytes)
	router.Use(RecoveryMiddleware[T](conf.ErrorHandler))

	return &App[T]{
		frozen: false,
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
func (a *App[T]) AddCron(name string, interval time.Duration, job CronJob) {
	if a.frozen {
		a.config.Logger.Error("Cannot add cronjobs after Aether is listening")
		return
	}
	a.cron.Add(name, interval, job)
}

// Listen starts the Aether HTTP server and handles graceful shutdown.
func (a *App[T]) Listen() error {
	a.frozen = true

	a.cron.Start()

	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)

	a.config.Logger.Infof("Aether is up and flying! %s", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if a.config.Timeout > 0 {
		t := time.Duration(a.config.Timeout) * time.Second
		srv.ReadTimeout = t
		srv.WriteTimeout = t
		srv.IdleTimeout = t * 2
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.config.Logger.Fatalf("Aether core listener failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.config.Logger.Warn("Aether is gently shutting down... Waiting for connections to finish.")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		a.config.Logger.Errorf("Aether forced to violently shutdown: %v", err)
	}

	a.cron.Stop()

	a.config.Logger.Info("Aether has successfully shutdown. See you soon!")
	a.config.Logger.Sync()

	return nil
}
