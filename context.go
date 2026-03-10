package aether

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ResponseWriter interface {
	http.ResponseWriter
	Status() int
	Size() int
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

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

func (rw *responseWriter) Status() int {
	if rw.status == 0 {
		return http.StatusOK
	}
	return rw.status
}

func (rw *responseWriter) Size() int {
	return rw.size
}

type Context[T any] struct {
	ctx      context.Context
	req      *http.Request
	res      ResponseWriter
	json     JSONEngine
	xml      XMLEngine
	Log      Logger
	handlers []HandlerFunc[T]
	index    int
	start    time.Time
	Global   T
}

func (c *Context[T]) Reset(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc[T], json JSONEngine, xml XMLEngine, log Logger, global T) {
	c.ctx = req.Context()
	c.req = req
	c.res = &responseWriter{ResponseWriter: w}
	c.json = json
	c.xml = xml
	c.Log = log
	c.handlers = handlers
	c.index = -1
	c.start = time.Now()
	c.Global = global
}



func (c *Context[T]) Param(key string) string {
	return c.req.PathValue(key)
}

func (c *Context[T]) Query(key string) string {
	return c.req.URL.Query().Get(key)
}

func (c *Context[T]) GetBody() ([]byte, error) {
	if c.req.Body == nil {
		return nil, nil
	}
	bodyBytes, err := io.ReadAll(c.req.Body)
	if err != nil {
		return nil, err
	}
	c.req.Body.Close()
	c.req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

func (c *Context[T]) SaveFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func (c *Context[T]) Next() {
	c.index++
	if c.index < len(c.handlers) {
		c.handlers[c.index](c)
	}
}

func (c *Context[T]) JSON(status int, data any) error {
	c.res.Header().Set("Content-Type", "application/json")
	c.res.WriteHeader(status)
	return c.json.Encode(c.res, data)
}

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

func (c *Context[T]) Bind(v any) error {
	contentType := c.req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		body, err := c.GetBody()
		if err != nil {
			return err
		}
		return c.json.Decode(bytes.NewReader(body), v)
	}

	if strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml") {
		body, err := c.GetBody()
		if err != nil {
			return err
		}
		return c.xml.Decode(bytes.NewReader(body), v)
	}

	if strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data") {
		if strings.Contains(contentType, "multipart/form-data") {
			if err := c.req.ParseMultipartForm(32 << 20); err != nil {
				return err
			}
		} else if err := c.req.ParseForm(); err != nil {
			return err
		}

		val := reflect.ValueOf(v)
		if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
			return fmt.Errorf("Aether: Bind for form data requires a pointer to struct")
		}
		
		elem := val.Elem()
		typ := elem.Type()
		fields := getFormMappings(typ)
		
		for _, f := range fields {
			if formValues := c.req.Form[f.Name]; len(formValues) > 0 {
				fieldValue := elem.Field(f.Index)
				if fieldValue.CanSet() {
					formValue := formValues[0]
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
						if fieldValue.Type().Elem().Kind() == reflect.String {
							sliceVal := reflect.MakeSlice(fieldValue.Type(), len(formValues), len(formValues))
							for i, v := range formValues {
								sliceVal.Index(i).SetString(v)
							}
							fieldValue.Set(sliceVal)
						}
					}
				}
			}
		}
		return nil
	}

	body, err := c.GetBody()
	if err != nil {
		return err
	}
	return c.json.Decode(bytes.NewReader(body), v)
}

type HandlerFunc[T any] func(c *Context[T])

type HandlerWithBody[T any, B any] func(c *Context[T], body B)

type CustomContext[T any, L any] struct {
	*Context[T]
	Data L
}

func WithCustomContext[T any, L any](initialData L, handler func(c *CustomContext[T, L])) HandlerFunc[T] {
	return func(c *Context[T]) {
		custom := &CustomContext[T, L]{
			Context: c,
			Data:    initialData,
		}
		handler(custom)
	}
}
