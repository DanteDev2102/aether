package main

import (
	"net/http"
)

type Router struct {
	mux    *http.ServeMux
	prefix string
	json   JSONEngine
	xml    XMLEngine
	log    Logger
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func NewGroup(prefix string, router *Router) *Router {
	return &Router{
		mux:    router.mux,
		prefix: router.prefix + prefix,
		json:   router.json,
		xml:    router.xml,
		log:    router.log,
	}
}

func NewRouter(jsonEngine JSONEngine, xmlEngine XMLEngine, log Logger) *Router {
	return &Router{
		mux:    http.NewServeMux(),
		prefix: "",
		json:   jsonEngine,
		xml:    xmlEngine,
		log:    log,
	}
}

func Get(r *Router, path string, h HandlerFunc) {
	fullPath := "GET " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		h(&Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log})
	})
}

func Delete(r *Router, path string, h HandlerFunc) {
	fullPath := "DELETE " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		h(&Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log})
	})
}

func Post[T any](r *Router, path string, h HandlerWithBody[T]) {
	fullPath := "POST " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		c := &Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log}
		var body T
		if err := c.Bind(&body); err != nil {
			http.Error(w, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Put[T any](r *Router, path string, h HandlerWithBody[T]) {
	fullPath := "PUT " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		c := &Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log}
		var body T
		if err := c.Bind(&body); err != nil {
			http.Error(w, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Patch[T any](r *Router, path string, h HandlerWithBody[T]) {
	fullPath := "PATCH " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		c := &Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log}
		var body T
		if err := c.Bind(&body); err != nil {
			http.Error(w, "Aether: Invalid Request Body", http.StatusBadRequest)
			return
		}
		h(c, body)
	})
}

func Head(r *Router, path string, h HandlerFunc) {
	fullPath := "HEAD " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		h(&Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log})
	})
}

func Connect(r *Router, path string, h HandlerFunc) {
	fullPath := "CONNECT " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		h(&Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log})
	})
}

func Options(r *Router, path string, h HandlerFunc) {
	fullPath := "OPTIONS " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		h(&Context{req: req, res: w, ctx: req.Context(), json: r.json, xml: r.xml, Log: r.log})
	})
}

func Trace(r *Router, path string, h HandlerFunc) {
	fullPath := "TRACE " + r.prefix + path

	r.mux.HandleFunc(fullPath, func(w http.ResponseWriter, req *http.Request) {
		h(&Context{req: req, res: w, ctx: req.Context(), json: r.json, Log: r.log})
	})
}
