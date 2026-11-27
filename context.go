package aether

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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
	Written() bool
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

// WriteString writes s without converting it to a byte slice when the
// underlying writer supports it.
func (rw *responseWriter) WriteString(s string) (int, error) {
	if rw.status == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := io.WriteString(rw.ResponseWriter, s)
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

// Flush sends any buffered data to the client. It records an implicit 200
// status first, so streaming responses report the right status and do not
// trigger a duplicate WriteHeader.
func (rw *responseWriter) Flush() {
	if rw.status == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(rw.ResponseWriter).Flush()
}

// Written reports whether the response status has already been sent.
func (rw *responseWriter) Written() bool {
	return rw.status != 0
}

// Unwrap returns the underlying http.ResponseWriter.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Context holds the request context for a handler.
//
// A Context is reused for later requests once the handler chain returns, so it
// must not be kept or used from goroutines after that; use Copy instead.
type Context[T any] struct {
	deadline *deadlineCtx
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
	// rw is the pooled response writer behind res, reused across requests.
	rw responseWriter
	// Global points to the application-wide state from Config.Global. It is
	// shared by all requests: synchronize any mutation (sync.Mutex, atomic).
	Global *T
}

// reset initializes the context for a new request.
func (c *Context[T]) reset(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc[T], json JSONEngine, xml XMLEngine, template TemplateEngine, cache CacheStore, log Logger, global *T) {
	c.deadline = nil
	c.req = req
	c.rw = responseWriter{ResponseWriter: w}
	c.res = &c.rw
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

// release drops references held by the context so pooled contexts do not keep
// requests, responses or global state alive between requests.
func (c *Context[T]) release() {
	c.deadline = nil
	c.req = nil
	c.res = nil
	c.rw = responseWriter{}
	c.handlers = nil
	c.Global = nil
}

// ClearTimeout removes the Config.Timeout deadline from the request context,
// for responses meant to run longer such as streams, downloads or WebSockets.
// Client disconnects still cancel the context. It is a no-op when no timeout
// is configured.
func (c *Context[T]) ClearTimeout() {
	if c.deadline != nil {
		c.deadline.clear()
	}
}

// ErrDetachedContext is returned when writing a response from a Context
// obtained with Copy.
var ErrDetachedContext = errors.New("aether: cannot write a response from a copied context")

// Copy returns a copy of the context that is safe to use from goroutines and
// after the handler returns, for example for background work. It keeps the
// request (whose context is no longer canceled when the request ends), path
// parameters, Global, logger and cache. It cannot write a response, and the
// request body must not be read from it.
func (c *Context[T]) Copy() *Context[T] {
	return &Context[T]{
		req:      c.req.WithContext(context.WithoutCancel(c.req.Context())),
		res:      &detachedWriter{header: http.Header{}},
		json:     c.json,
		xml:      c.xml,
		template: c.template,
		cache:    c.cache,
		log:      c.log,
		start:    c.start,
		Global:   c.Global,
	}
}

// detachedWriter is the response writer of a copied context.
type detachedWriter struct {
	header http.Header
}

// Header returns a header map that is never sent.
func (d *detachedWriter) Header() http.Header { return d.header }

// Write always fails with ErrDetachedContext.
func (d *detachedWriter) Write([]byte) (int, error) { return 0, ErrDetachedContext }

// WriteHeader does nothing.
func (d *detachedWriter) WriteHeader(int) {}

// Status returns 0: a copied context has no response.
func (d *detachedWriter) Status() int { return 0 }

// Size returns 0.
func (d *detachedWriter) Size() int { return 0 }

// Written reports true so nothing tries to write a response.
func (d *detachedWriter) Written() bool { return true }

// Unwrap returns nil: there is no underlying writer.
func (d *detachedWriter) Unwrap() http.ResponseWriter { return nil }

// Req returns the current HTTP request.
func (c *Context[T]) Req() *http.Request {
	return c.req
}

// SetReq sets the current HTTP request.
func (c *Context[T]) SetReq(req *http.Request) {
	c.req = req
}

// Res returns the current HTTP response writer. The returned value always
// implements ResponseWriter.
func (c *Context[T]) Res() http.ResponseWriter {
	return c.res
}

// SetRes replaces the response writer used by the context. Middlewares use it
// to wrap the response, for example to compress it.
func (c *Context[T]) SetRes(w ResponseWriter) {
	c.res = w
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

// JSON writes a JSON response with the specified status code. The value is
// encoded before anything is written, so an encoding error leaves the response
// untouched and the caller can still send an error response.
func (c *Context[T]) JSON(status int, data any) error {
	return c.encoded(status, contentTypeJSON, c.json.Encode, data)
}

// XML writes an XML response with the specified status code. Like JSON, it
// encodes the value before writing anything.
func (c *Context[T]) XML(status int, data any) error {
	return c.encoded(status, contentTypeXML, c.xml.Encode, data)
}

func (c *Context[T]) encoded(status int, contentType []string, encode func(io.Writer, any) error, data any) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := encode(buf, data); err != nil {
		return err
	}

	c.res.Header()["Content-Type"] = contentType
	c.res.WriteHeader(status)
	_, err := c.res.Write(buf.Bytes())
	return err
}

// Shared Content-Type header values. Assigning them directly avoids allocating
// a new slice per response; net/http only reads them.
var (
	contentTypeJSON  = []string{"application/json"}
	contentTypeXML   = []string{"application/xml"}
	contentTypePlain = []string{"text/plain; charset=utf-8"}
	contentTypeHTML  = []string{"text/html; charset=utf-8"}
)

// String writes text as a plain text response with the specified status code.
// The text is written as is; use Stringf to format it.
func (c *Context[T]) String(status int, text string) error {
	c.res.Header()["Content-Type"] = contentTypePlain
	c.res.WriteHeader(status)
	_, err := io.WriteString(c.res, text)
	return err
}

// Stringf formats according to a format specifier and writes the result as a
// plain text response with the specified status code.
func (c *Context[T]) Stringf(status int, format string, args ...any) error {
	return c.String(status, fmt.Sprintf(format, args...))
}

// Render renders a template with the specified name and data.
func (c *Context[T]) Render(status int, name string, data any) error {
	if c.template == nil {
		return fmt.Errorf("aether: template engine is not configured")
	}
	c.res.Header()["Content-Type"] = contentTypeHTML
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

// SSE prepares the response for Server-Sent Events. It removes the request
// timeout (see ClearTimeout) so the stream is not cut after Config.Timeout.
func (c *Context[T]) SSE() (*http.ResponseController, error) {
	c.ClearTimeout()
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
		if !field.IsExported() {
			continue
		}
		tag, _, _ := strings.Cut(field.Tag.Get("form"), ",")
		if tag == "" {
			tag, _, _ = strings.Cut(field.Tag.Get("json"), ",")
		}
		if tag == "-" {
			continue
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
// JSON and XML bodies are decoded as they are read, so the body is consumed;
// call GetBody first if the raw body is also needed.
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

// bindJSON decodes the JSON body into v, streaming from the request body. An
// empty body is not an error and leaves v unchanged.
func (c *Context[T]) bindJSON(v any) error {
	return c.decodeBody(c.json.Decode, v)
}

// bindXML decodes the XML body into v, streaming from the request body. An
// empty body is not an error and leaves v unchanged.
func (c *Context[T]) bindXML(v any) error {
	return c.decodeBody(c.xml.Decode, v)
}

func (c *Context[T]) decodeBody(decode func(io.Reader, any) error, v any) error {
	body := c.req.Body
	if body == nil || body == http.NoBody {
		return nil
	}
	if err := decode(body, v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
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
		formValues := c.req.Form[f.Name]
		if len(formValues) == 0 {
			continue
		}
		fieldValue := elem.Field(f.Index)
		if !fieldValue.CanSet() {
			continue
		}
		if err := setFormField(fieldValue, formValues); err != nil {
			return fmt.Errorf("aether: invalid value for form field %q: %w", f.Name, err)
		}
	}
	return nil
}

// setFormField assigns form values to a struct field. Slices receive every
// value; other kinds receive the first one.
func setFormField(field reflect.Value, values []string) error {
	if field.Kind() != reflect.Slice {
		return setFormValue(field, values[0])
	}
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, v := range values {
		if err := setFormValue(slice.Index(i), v); err != nil {
			return err
		}
	}
	field.Set(slice)
	return nil
}

// setFormValue parses value into a field of a basic kind. Unsupported kinds
// are left untouched.
func setFormValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	}
	return nil
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
// When the middleware passes its own http.ResponseWriter to the next handler
// (to compress, buffer or measure the response), later handlers write through
// it; the original writer is restored once the chain returns.
func WrapMiddleware[T any](mw func(http.Handler) http.Handler) HandlerFunc[T] {
	return func(c *Context[T]) {
		orig := c.res
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.SetReq(r)
			if w != http.ResponseWriter(orig) {
				if rw, ok := w.(ResponseWriter); ok {
					c.SetRes(rw)
				} else {
					c.SetRes(&responseWriter{ResponseWriter: w})
				}
			}
			c.Next()
		})

		mw(nextHandler).ServeHTTP(orig, c.Req())
		c.SetRes(orig)
	}
}
