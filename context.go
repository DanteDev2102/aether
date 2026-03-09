package main

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type Context struct {
	ctx      context.Context
	req      *http.Request
	res      http.ResponseWriter
	json     JSONEngine
	xml      XMLEngine
	Log      Logger
	handlers []HandlerFunc
	index    int
}

func (c *Context) Next() {
	c.index++
	if c.index < len(c.handlers) {
		c.handlers[c.index](c)
	}
}

func (c *Context) JSON(status int, data any) error {
	c.res.Header().Set("Content-Type", "application/json")
	c.res.WriteHeader(status)
	return c.json.Encode(c.res, data)
}

func (c *Context) XML(status int, data any) error {
	c.res.Header().Set("Content-Type", "application/xml")
	c.res.WriteHeader(status)
	return c.xml.Encode(c.res, data)
}

func (c *Context) String(status int, text string, args ...any) error {
	c.res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.res.WriteHeader(status)
	_, err := fmt.Fprintf(c.res, text, args...)
	return err
}

func (c *Context) Bind(v any) error {
	contentType := c.req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		return c.json.Decode(c.req.Body, v)
	}

	if strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml") {
		return c.xml.Decode(c.req.Body, v)
	}

	if strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data") {
		if err := c.req.ParseForm(); err != nil {
			return err
		}

		val := reflect.ValueOf(v)
		if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
			return fmt.Errorf("Bind for form data requires a pointer to struct")
		}
		
		elem := val.Elem()
		typ := elem.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag := field.Tag.Get("form")
			if tag == "" {
				tag = field.Tag.Get("json")
			}
			if tag == "" {
				tag = strings.ToLower(field.Name)
			}
			
			if formValue := c.req.FormValue(tag); formValue != "" {
				fieldValue := elem.Field(i)
				if fieldValue.CanSet() {
					switch fieldValue.Kind() {
					case reflect.String:
						fieldValue.SetString(formValue)
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						if intVal, err := strconv.ParseInt(formValue, 10, 64); err == nil {
							fieldValue.SetInt(intVal)
						}
					case reflect.Bool:
						if boolVal, err := strconv.ParseBool(formValue); err == nil {
							fieldValue.SetBool(boolVal)
						}
					}
				}
			}
		}
		return nil
	}

	return c.json.Decode(c.req.Body, v)
}

type HandlerFunc func(c *Context)

type HandlerWithBody[T any] func(c *Context, body T)
