package aether

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Router[T any] struct {
	mux          *http.ServeMux
	prefix       string
	json         JSONEngine
	xml          XMLEngine
	template     TemplateEngine
	cache        CacheStore
	log          Logger
	middlewares  []HandlerFunc[T]
	ctxPool      sync.Pool
	global       T
	timeout      int
	maxBodyBytes int64
}

func (r *Router[T]) Use(middlewares ...HandlerFunc[T]) {
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *Router[T]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

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
		ctxPool: sync.Pool{
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

		c := r.ctxPool.Get().(*Context[T])
		c.Reset(w, req, chain, r.json, r.xml, r.template, r.cache, r.log, r.global)

		c.Next()

		r.ctxPool.Put(c)
	})
}

func Get[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "GET", path, h)
}

func Delete[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "DELETE", path, h)
}

func Post[T any, B any](r *Router[T], path string, h HandlerWithBody[T, B]) {
	registerHelper(r, "POST", path, func(c *Context[T]) {
		var body B
		if err := c.Bind(&body); err != nil {
			http.Error(c.res, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Put[T any, B any](r *Router[T], path string, h HandlerWithBody[T, B]) {
	registerHelper(r, "PUT", path, func(c *Context[T]) {
		var body B
		if err := c.Bind(&body); err != nil {
			http.Error(c.res, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Patch[T any, B any](r *Router[T], path string, h HandlerWithBody[T, B]) {
	registerHelper(r, "PATCH", path, func(c *Context[T]) {
		var body B
		if err := c.Bind(&body); err != nil {
			http.Error(c.res, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Head[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "HEAD", path, h)
}

func Connect[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "CONNECT", path, h)
}

func Options[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "OPTIONS", path, h)
}

func Trace[T any](r *Router[T], path string, h HandlerFunc[T]) {
	registerHelper(r, "TRACE", path, h)
}

func Static[T any](r *Router[T], pathPrefix string, rootFolder string) {
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}

	fullPrefix := r.prefix + pathPrefix
	fs := http.StripPrefix(fullPrefix, http.FileServer(http.Dir(rootFolder)))

	routePath := pathPrefix + "{filepath...}"

	registerHelper(r, "GET", routePath, func(c *Context[T]) {
		fs.ServeHTTP(c.res, c.req)
	})
}
