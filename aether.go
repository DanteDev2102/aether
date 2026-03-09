package main

import (
	"fmt"
	"net/http"
	"time"
)

type Config struct {
	Port    int
	Host    string
	Timeout int
	JSON    JSONEngine
	XML     XMLEngine
	Logger  Logger
}

type App struct {
	frozen bool
	config *Config
	router *Router
	cron   *CronManager
}

func New(conf *Config) *App {
	if conf.JSON == nil {
		conf.JSON = stdJSONEngine{}
	}
	if conf.XML == nil {
		conf.XML = stdXMLEngine{}
	}
	if conf.Logger == nil {
		conf.Logger = newStdLogger()
	}
	router := NewRouter(conf.JSON, conf.XML, conf.Logger)
	router.Use(LoggerMiddleware())

	return &App{
		frozen: false,
		config: conf,
		router: router,
		cron:   newCronManager(conf.Logger),
	}
}

func (a *App) Router() *Router {
	return a.router
}

func (a *App) AddCron(name string, interval time.Duration, job CronJob) {
	if a.frozen {
		a.config.Logger.Error("Cannot add cronjobs after Aether is listening")
		return
	}
	a.cron.Add(name, interval, job)
}

func (a *App) Listen() error {
	a.frozen = true

	a.cron.Start()
	
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)

	a.config.Logger.Infof("Aether is up and flying! %s", addr)

	return http.ListenAndServe(addr, a.router)
}
