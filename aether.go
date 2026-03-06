package aether

import (
	"fmt"
	"context"
	"net/http"
)

type Config struct {
	Port    int
	Host    string
	Timeout int
}

type Router struct {
	mux *http.ServeMux
}

type App struct {
	frozen bool
	config *Config
	router *Router
}

type Context struct {
	ctx *context.Context
	req *http.Request
	res *http.ResponseWriter
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func NewRouter() *Router {
    return &Router{ mux: http.NewServeMux() }
}

func New(conf *Config) *App {
	return &App{
		frozen: false,
		config: conf,
		router: NewRouter(),
	}
}

func (a *App) Listen() error {
	a.frozen = true

	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)

	return http.ListenAndServe(addr, a.router)
}

