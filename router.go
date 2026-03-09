package main

import (
	"net/http"
)

type Router struct {
	mux         *http.ServeMux
	prefix      string
	json        JSONEngine
	xml         XMLEngine
	log         Logger
	middlewares []HandlerFunc
}

func (r *Router) Use(middlewares ...HandlerFunc) {
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func NewGroup(prefix string, router *Router) *Router {
	m := make([]HandlerFunc, len(router.middlewares))
	copy(m, router.middlewares)

	return &Router{
		mux:         router.mux,
		prefix:      router.prefix + prefix,
		json:        router.json,
		xml:         router.xml,
		log:         router.log,
		middlewares: m,
	}
}

func NewRouter(jsonEngine JSONEngine, xmlEngine XMLEngine, log Logger) *Router {
	return &Router{
		mux:         http.NewServeMux(),
		prefix:      "",
		json:        jsonEngine,
		xml:         xmlEngine,
		log:         log,
		middlewares: make([]HandlerFunc, 0),
	}
}

func registerHelper(r *Router, method, path string, finalHandler HandlerFunc) {
	fullPath := method + " " + r.prefix + path

	chain := make([]HandlerFunc, len(r.middlewares)+1)
	copy(chain, r.middlewares)
	chain[len(r.middlewares)] = finalHandler

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		c := &Context{
			req:      req,
			res:      w,
			ctx:      req.Context(),
			json:     r.json,
			xml:      r.xml,
			Log:      r.log,
			handlers: chain,
			index:    -1,
		}
		c.Next()
	})
}

func Get(r *Router, path string, h HandlerFunc) {
	registerHelper(r, "GET", path, h)
}

func Delete(r *Router, path string, h HandlerFunc) {
	registerHelper(r, "DELETE", path, h)
}

func Post[T any](r *Router, path string, h HandlerWithBody[T]) {
	registerHelper(r, "POST", path, func(c *Context) {
		var body T
		if err := c.Bind(&body); err != nil {
			http.Error(c.res, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Put[T any](r *Router, path string, h HandlerWithBody[T]) {
	registerHelper(r, "PUT", path, func(c *Context) {
		var body T
		if err := c.Bind(&body); err != nil {
			http.Error(c.res, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Patch[T any](r *Router, path string, h HandlerWithBody[T]) {
	registerHelper(r, "PATCH", path, func(c *Context) {
		var body T
		if err := c.Bind(&body); err != nil {
			http.Error(c.res, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Head(r *Router, path string, h HandlerFunc) {
	registerHelper(r, "HEAD", path, h)
}

func Connect(r *Router, path string, h HandlerFunc) {
	registerHelper(r, "CONNECT", path, h)
}

func Options(r *Router, path string, h HandlerFunc) {
	registerHelper(r, "OPTIONS", path, h)
}

func Trace(r *Router, path string, h HandlerFunc) {
	registerHelper(r, "TRACE", path, h)
}
