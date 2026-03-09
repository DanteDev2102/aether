package main

import (
	"net/http"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LoggerMiddleware() HandlerFunc {
	return func(c *Context) {
		start := time.Now()
		
		lrw := &loggingResponseWriter{ResponseWriter: c.res, statusCode: http.StatusOK}
		c.res = lrw
		
		c.Next()
		
		duration := time.Since(start)
		c.Log.Infof("%s %s | %d | %v", c.req.Method, c.req.URL.Path, lrw.statusCode, duration)
	}
}
