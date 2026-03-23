package aether

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ResponseWriter wraps http.ResponseWriter with status and size tracking.
type ResponseWriter interface {
	http.ResponseWriter
	Status() int
	Size() int
	Unwrap() http.ResponseWriter
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader captures the response status code.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.status == 0 {
		rw.status = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Status returns the current response status code.
func (rw *responseWriter) Status() int {
	if rw.status == 0 {
		return http.StatusOK
	}
	return rw.status
}

// Size returns the number of bytes written.
func (rw *responseWriter) Size() int {
	return rw.size
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Context holds the request context for a handler.
type Context[T any] struct {
	ctx      context.Context
	req      *http.Request
	res      ResponseWriter
	json     JSONEngine
	xml      XMLEngine
	template TemplateEngine
	cache    CacheStore
	log      Logger
	handlers []HandlerFunc[T]
	index    int
	start    time.Time
	Global   T
}

// Reset initializes or resets the context with new values.
func (c *Context[T]) Reset(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc[T], json JSONEngine, xml XMLEngine, template TemplateEngine, cache CacheStore, log Logger, global T) {
	c.ctx = req.Context()
	c.req = req
	c.res = &responseWriter{ResponseWriter: w}
	c.json = json
	c.xml = xml
	c.template = template
	c.cache = cache
	c.log = log
	c.handlers = handlers
	c.index = -1
	c.start = time.Now()
	c.Global = global
}

// Req returns the current HTTP request.
func (c *Context[T]) Req() *http.Request {
	return c.req
}

// SetReq sets the current HTTP request.
func (c *Context[T]) SetReq(req *http.Request) {
	c.req = req
}

// Res returns the current HTTP response writer.
func (c *Context[T]) Res() http.ResponseWriter {
	return c.res
}

// Log returns the logger instance.
func (c *Context[T]) Log() Logger {
	return c.log
}

// Start returns the time when the request started.
func (c *Context[T]) Start() time.Time {
	return c.start
}

// Cache returns the cache store instance.
func (c *Context[T]) Cache() CacheStore {
	return c.cache
}

// Param returns the path parameter value by key.
func (c *Context[T]) Param(key string) string {
	return c.req.PathValue(key)
}

// Query returns the query parameter value by key.
func (c *Context[T]) Query(key string) string {
	return c.req.URL.Query().Get(key)
}

// GetBody reads and returns the request body.
func (c *Context[T]) GetBody() ([]byte, error) {
	if c.req.Body == nil {
		return nil, nil
	}
	bodyBytes, err := io.ReadAll(c.req.Body)
	if err != nil {
		return nil, err
	}
	_ = c.req.Body.Close()
	c.req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

// SaveFile saves an uploaded file to the specified destination.
func (c *Context[T]) SaveFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	if err = os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, src)
	return err
}

// Next executes the next handler in the middleware chain.
func (c *Context[T]) Next() {
	c.index++
	if c.index < len(c.handlers) {
		c.handlers[c.index](c)
	}
}

// JSON writes a JSON response with the specified status code.
func (c *Context[T]) JSON(status int, data any) error {
	c.res.Header().Set("Content-Type", "application/json")
	c.res.WriteHeader(status)
	return c.json.Encode(c.res, data)
}

// XML writes an XML response with the specified status code.
func (c *Context[T]) XML(status int, data any) error {
	c.res.Header().Set("Content-Type", "application/xml")
	c.res.WriteHeader(status)
	return c.xml.Encode(c.res, data)
}

func (c *Context[T]) String(status int, text string, args ...any) error {
	c.res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.res.WriteHeader(status)
	_, err := fmt.Fprintf(c.res, text, args...)
	return err
}

// Render renders a template with the specified name and data.
func (c *Context[T]) Render(status int, name string, data any) error {
	if c.template == nil {
		return fmt.Errorf("aether: template engine is not configured")
	}
	c.res.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.res.WriteHeader(status)
	return c.template.Render(c.res, name, data)
}

// SetCookie sets a cookie in the response.
func (c *Context[T]) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.res, cookie)
}

// Cookie returns the cookie with the specified name.
func (c *Context[T]) Cookie(name string) (*http.Cookie, error) {
	return c.req.Cookie(name)
}

// ClearCookie removes a cookie by setting its max age to -1.
func (c *Context[T]) ClearCookie(name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-100 * time.Hour),
		HttpOnly: true,
	}
	http.SetCookie(c.res, cookie)
}

// File serves the file at the specified path.
func (c *Context[T]) File(filepath string) {
	http.ServeFile(c.res, c.req, filepath)
}

// Attachment serves a file as an attachment with the specified filename.
func (c *Context[T]) Attachment(filepath, filename string) {
	c.res.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(c.res, c.req, filepath)
}

// SSE prepares the response for Server-Sent Events.
func (c *Context[T]) SSE() (*http.ResponseController, error) {
	c.res.Header().Set("Content-Type", "text/event-stream")
	c.res.Header().Set("Cache-Control", "no-cache")
	c.res.Header().Set("Connection", "keep-alive")
	rc := http.NewResponseController(c.res)
	err := rc.Flush()
	return rc, err
}

// Hijack takes control of the underlying connection.
func (c *Context[T]) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rc := http.NewResponseController(c.res)
	return rc.Hijack()
}

type formField struct {
	Index int
	Name  string
}

var formStructCache sync.Map

func getFormMappings(typ reflect.Type) []formField {
	if cached, ok := formStructCache.Load(typ); ok {
		return cached.([]formField)
	}
	var fields []formField
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("form")
		if tag == "" {
			tag = field.Tag.Get("json")
		}
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}
		fields = append(fields, formField{Index: i, Name: tag})
	}
	formStructCache.Store(typ, fields)
	return fields
}

// Bind binds request body data to the specified struct based on content type.
func (c *Context[T]) Bind(v any) error {
	contentType := c.req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		return c.bindJSON(v)
	}

	if strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml") {
		return c.bindXML(v)
	}

	if strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data") {
		return c.bindForm(v, strings.Contains(contentType, "multipart/form-data"))
	}

	return c.bindJSON(v)
}

func (c *Context[T]) bindJSON(v any) error {
	body, err := c.GetBody()
	if err != nil {
		return err
	}
	return c.json.Decode(bytes.NewReader(body), v)
}

func (c *Context[T]) bindXML(v any) error {
	body, err := c.GetBody()
	if err != nil {
		return err
	}
	return c.xml.Decode(bytes.NewReader(body), v)
}

func (c *Context[T]) bindForm(v any, isMultipart bool) error {
	if isMultipart {
		if err := c.req.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
	} else if err := c.req.ParseForm(); err != nil {
		return err
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("aether: bind for form data requires a pointer to struct")
	}

	elem := val.Elem()
	typ := elem.Type()
	fields := getFormMappings(typ)

	for _, f := range fields {
		if formValues := c.req.Form[f.Name]; len(formValues) > 0 {
			fieldValue := elem.Field(f.Index)
			if fieldValue.CanSet() {
				c.setFieldValue(fieldValue, formValues[0])
			}
		}
	}
	return nil
}

func (c *Context[T]) setFieldValue(fieldValue reflect.Value, formValue string) {
	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(formValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intVal, err := strconv.ParseInt(formValue, 10, 64); err == nil {
			fieldValue.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintVal, err := strconv.ParseUint(formValue, 10, 64); err == nil {
			fieldValue.SetUint(uintVal)
		}
	case reflect.Float32, reflect.Float64:
		if floatVal, err := strconv.ParseFloat(formValue, 64); err == nil {
			fieldValue.SetFloat(floatVal)
		}
	case reflect.Bool:
		if boolVal, err := strconv.ParseBool(formValue); err == nil {
			fieldValue.SetBool(boolVal)
		}
	case reflect.Slice:
		c.setSliceValue(fieldValue, formValue)
	}
}

func (c *Context[T]) setSliceValue(fieldValue reflect.Value, formValue string) {
	if fieldValue.Type().Elem().Kind() != reflect.String {
		return
	}
	sliceVal := reflect.MakeSlice(fieldValue.Type(), 1, 1)
	sliceVal.Index(0).SetString(formValue)
	fieldValue.Set(sliceVal)
}

// HandlerFunc is the function signature for HTTP handlers.
type HandlerFunc[T any] func(c *Context[T])

// HandlerWithBody is the function signature for handlers that receive a request body.
type HandlerWithBody[T any, B any] func(c *Context[T], body B)

// CustomContext extends Context with custom user data.
type CustomContext[T any, L any] struct {
	*Context[T]
	Data L
}

// WithCustomContext creates a handler with custom context data.
func WithCustomContext[T, L any](initialData L, handler func(c *CustomContext[T, L])) HandlerFunc[T] {
	return func(c *Context[T]) {
		custom := &CustomContext[T, L]{
			Context: c,
			Data:    initialData,
		}
		handler(custom)
	}
}

// WrapMiddleware adapts a standard net/http middleware into an Aether HandlerFunc.
func WrapMiddleware[T any](mw func(http.Handler) http.Handler) HandlerFunc[T] {
	return func(c *Context[T]) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.SetReq(r)
			c.Next()
		})

		mw(nextHandler).ServeHTTP(c.Res(), c.Req())
	}
}
