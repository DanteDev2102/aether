package aether

import (
	"errors"
	"fmt"
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
	global       *T
	timeout      time.Duration
	maxBodyBytes int64
	sealed       bool
}

// Use registers one or more middleware handlers. Middlewares apply to every
// route registered on this router and to groups created from it, so Use must
// be called before registering routes or creating groups; otherwise it panics
// instead of silently skipping the routes that already exist.
func (r *Router[T]) Use(middlewares ...HandlerFunc[T]) {
	if r.sealed {
		panic(fmt.Sprintf("aether: Use called on router %q after registering routes or creating groups; call Use first", r.prefixOrRoot()))
	}
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *Router[T]) prefixOrRoot() string {
	if r.prefix == "" {
		return "/"
	}
	return r.prefix
}

// ServeHTTP dispatches the request to the matching route. Requests that do not
// match any route (404, 405, CORS preflight) still run through the router's
// middlewares before the default response is written, so global middlewares
// such as CORS or recovery apply to them as well.
func (r *Router[T]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// fallbackPattern matches every method and path with the lowest precedence in
// http.ServeMux, so it only receives requests no route matched. Handling them
// here keeps a single route lookup per request.
const fallbackPattern = "/"

var routeMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace,
}

// serveUnmatched runs the root middlewares for a request that matched no
// route, then answers 405 with an Allow header when the path exists for other
// methods, or 404 otherwise.
func (r *Router[T]) serveUnmatched(w http.ResponseWriter, req *http.Request) {
	chain := make([]HandlerFunc[T], len(r.middlewares)+1)
	copy(chain, r.middlewares)
	chain[len(r.middlewares)] = func(c *Context[T]) {
		if allow := r.allowedMethods(c.Req()); len(allow) > 0 {
			c.Res().Header().Set("Allow", strings.Join(allow, ", "))
			http.Error(c.Res(), http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(c.Res(), c.Req())
	}
	r.serve(w, req, chain, nil)
}

// allowedMethods returns the methods that have a route for the request path.
func (r *Router[T]) allowedMethods(req *http.Request) []string {
	var allow []string
	probe := *req
	for _, m := range routeMethods {
		probe.Method = m
		if _, pattern := r.mux.Handler(&probe); pattern != fallbackPattern && pattern != "" {
			allow = append(allow, m)
		}
	}
	return allow
}

func (r *Router[T]) serve(w http.ResponseWriter, req *http.Request, chain []HandlerFunc[T], deadline *deadlineCtx) {
	c := r.ctxPool.Get().(*Context[T])
	c.reset(w, req, chain, r.json, r.xml, r.template, r.cache, r.log, r.global)
	c.deadline = deadline

	c.Next()

	c.release()
	r.ctxPool.Put(c)
}

// NewGroup creates a new router group with the specified prefix. The group
// inherits the parent's middlewares; after this call the parent no longer
// accepts Use.
func NewGroup[T any](prefix string, router *Router[T]) *Router[T] {
	router.sealed = true

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

// newRouter creates the root Router for an application.
func newRouter[T any](conf *Config[T]) *Router[T] {
	r := &Router[T]{
		mux:         http.NewServeMux(),
		prefix:      "",
		json:        conf.JSON,
		xml:         conf.XML,
		template:    conf.Template,
		cache:       conf.Cache,
		log:         conf.Logger,
		middlewares: make([]HandlerFunc[T], 0),
		ctxPool: &sync.Pool{
			New: func() any {
				return &Context[T]{}
			},
		},
		global:       &conf.Global,
		timeout:      conf.Timeout,
		maxBodyBytes: conf.MaxBodyBytes,
	}
	r.mux.HandleFunc(fallbackPattern, r.serveUnmatched)
	return r
}

func registerHelper[T any](r *Router[T], method, path string, finalHandler HandlerFunc[T]) {
	fullPath := method + " " + r.prefix + path
	r.sealed = true

	chain := make([]HandlerFunc[T], len(r.middlewares)+1)
	copy(chain, r.middlewares)
	chain[len(r.middlewares)] = finalHandler

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		var deadline *deadlineCtx
		if r.timeout > 0 {
			deadline = newDeadlineCtx(req.Context(), r.timeout)
			defer deadline.finish()
			req = req.WithContext(deadline)
		}

		if r.maxBodyBytes > 0 && req.Body != nil && req.Body != http.NoBody {
			req.Body = http.MaxBytesReader(w, req.Body, r.maxBodyBytes)
		}

		r.serve(w, req, chain, deadline)
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
