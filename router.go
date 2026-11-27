package aether

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Router handles HTTP routing and middleware registration.
type Router[T any] struct {
	mux          *http.ServeMux
	prefix       string
	json         JSONEngine
	xml          XMLEngine
	template     TemplateEngine
	cache        CacheStore
	log          Logger
	middlewares  []HandlerFunc[T]
	ctxPool      *sync.Pool
	global       T
	timeout      int
	maxBodyBytes int64
}

// Use registers one or more middleware handlers.
func (r *Router[T]) Use(middlewares ...HandlerFunc[T]) {
	r.middlewares = append(r.middlewares, middlewares...)
}

// ServeHTTP dispatches the request to the matching route. Requests that do not
// match any route (404, 405, CORS preflight) still run through the router's
// middlewares before the default response is written, so global middlewares
// such as CORS or recovery apply to them as well.
func (r *Router[T]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if _, pattern := r.mux.Handler(req); pattern != "" || len(r.middlewares) == 0 {
		r.mux.ServeHTTP(w, req)
		return
	}

	chain := make([]HandlerFunc[T], len(r.middlewares)+1)
	copy(chain, r.middlewares)
	chain[len(r.middlewares)] = func(c *Context[T]) {
		r.mux.ServeHTTP(c.Res(), c.Req())
	}

	r.serve(w, req, chain)
}

func (r *Router[T]) serve(w http.ResponseWriter, req *http.Request, chain []HandlerFunc[T]) {
	c := r.ctxPool.Get().(*Context[T])
	c.Reset(w, req, chain, r.json, r.xml, r.template, r.cache, r.log, r.global)

	c.Next()

	c.release()
	r.ctxPool.Put(c)
}

// NewGroup creates a new router group with the specified prefix.
func NewGroup[T any](prefix string, router *Router[T]) *Router[T] {
	m := make([]HandlerFunc[T], len(router.middlewares))
	copy(m, router.middlewares)

	return &Router[T]{
		mux:          router.mux,
		prefix:       router.prefix + prefix,
		json:         router.json,
		xml:          router.xml,
		template:     router.template,
		cache:        router.cache,
		log:          router.log,
		middlewares:  m,
		ctxPool:      router.ctxPool,
		global:       router.global,
		timeout:      router.timeout,
		maxBodyBytes: router.maxBodyBytes,
	}
}

// NewRouter creates a new Router instance with the given configuration.
func NewRouter[T any](jsonEngine JSONEngine, xmlEngine XMLEngine, templateEngine TemplateEngine, cacheStore CacheStore, log Logger, global T, timeout int, maxBodyBytes int64) *Router[T] {
	return &Router[T]{
		mux:         http.NewServeMux(),
		prefix:      "",
		json:        jsonEngine,
		xml:         xmlEngine,
		template:    templateEngine,
		cache:       cacheStore,
		log:         log,
		middlewares: make([]HandlerFunc[T], 0),
		ctxPool: &sync.Pool{
			New: func() any {
				return &Context[T]{}
			},
		},
		global:       global,
		timeout:      timeout,
		maxBodyBytes: maxBodyBytes,
	}
}

func registerHelper[T any](r *Router[T], method, path string, finalHandler HandlerFunc[T]) {
	fullPath := method + " " + r.prefix + path

	chain := make([]HandlerFunc[T], len(r.middlewares)+1)
	copy(chain, r.middlewares)
	chain[len(r.middlewares)] = finalHandler

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		if r.timeout > 0 {
			ctx, cancel := context.WithTimeout(req.Context(), time.Duration(r.timeout)*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
		}

		if r.maxBodyBytes > 0 {
			req.Body = http.MaxBytesReader(w, req.Body, r.maxBodyBytes)
		}

		r.serve(w, req, chain)
	})
}

// bindBody adapts a HandlerWithBody into a HandlerFunc that decodes the request
// body before calling h. An empty body yields the zero value of B. Bodies over
// the configured size limit get 413 and malformed bodies get 400.
func bindBody[T, B any](h HandlerWithBody[T, B]) HandlerFunc[T] {
	return func(c *Context[T]) {
		var body B
		if err := c.Bind(&body); err != nil {
			status := http.StatusBadRequest
			msg := "Invalid request body"
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				status = http.StatusRequestEntityTooLarge
				msg = "Request body too large"
			}
			_ = c.JSON(status, map[string]string{"error": msg})
			return
		}
		h(c, body)
	}
}

// Get registers a handler for GET requests at the specified path.
func Get[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "GET", path, h)
}

// Delete registers a handler for DELETE requests at the specified path.
func Delete[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "DELETE", path, h)
}

// Post registers a handler for POST requests at the specified path.
func Post[T, B any](r *Router[T], path string, h HandlerWithBody[T, B]) {
	registerHelper(r, "POST", path, bindBody(h))
}

// Put registers a handler for PUT requests at the specified path.
func Put[T, B any](r *Router[T], path string, h HandlerWithBody[T, B]) {
	registerHelper(r, "PUT", path, bindBody(h))
}

// Patch registers a handler for PATCH requests at the specified path.
func Patch[T, B any](r *Router[T], path string, h HandlerWithBody[T, B]) {
	registerHelper(r, "PATCH", path, bindBody(h))
}

// Head registers a handler for HEAD requests at the specified path.
func Head[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "HEAD", path, h)
}

// Connect registers a handler for CONNECT requests at the specified path.
func Connect[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "CONNECT", path, h)
}

// Options registers a handler for OPTIONS requests at the specified path.
func Options[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "OPTIONS", path, h)
}

// Trace registers a handler for TRACE requests at the specified path.
func Trace[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "TRACE", path, h)
}

// Static serves static files from the specified root folder.
func Static[T any](r *Router[T], pathPrefix, rootFolder string) {
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}

	fullPrefix := r.prefix + pathPrefix
	fs := http.StripPrefix(fullPrefix, http.FileServer(http.Dir(rootFolder)))

	routePath := pathPrefix + "{filepath...}"

	registerHelper(r, "GET", routePath, func(c *Context[T]) {
		fs.ServeHTTP(c.Res(), c.Req())
	})
}
