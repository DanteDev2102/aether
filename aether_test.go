package aether

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testGlobal struct {
	AppName string
}

func newTestApp() *App[testGlobal] {
	return New[testGlobal](&Config[testGlobal]{
		Port: 0,
		Global: testGlobal{AppName: "TestApp"},
	})
}

func TestGetRoute(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/hello", func(c *Context[testGlobal]) {
		c.String(http.StatusOK, "Hello, World!")
	})

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "Hello, World!" {
		t.Errorf("expected body 'Hello, World!', got '%s'", w.Body.String())
	}
}

func TestPostRoute(t *testing.T) {
	type reqBody struct {
		Name string `json:"name"`
	}

	app := newTestApp()
	r := app.Router()

	Post[testGlobal, reqBody](r, "/users", func(c *Context[testGlobal], body reqBody) {
		c.JSON(http.StatusCreated, map[string]string{"name": body.Name})
	})

	payload := `{"name":"Aether"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "Aether" {
		t.Errorf("expected name 'Aether', got '%s'", resp["name"])
	}
}

func TestContextJSON(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/json", func(c *Context[testGlobal]) {
		c.JSON(http.StatusOK, map[string]string{"key": "value"})
	})

	req := httptest.NewRequest("GET", "/json", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["key"] != "value" {
		t.Errorf("expected key 'value', got '%s'", resp["key"])
	}
}

func TestContextString(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/text", func(c *Context[testGlobal]) {
		c.String(http.StatusOK, "Hello %s", "Aether")
	})

	req := httptest.NewRequest("GET", "/text", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type 'text/plain; charset=utf-8', got '%s'", ct)
	}
	if w.Body.String() != "Hello Aether" {
		t.Errorf("expected body 'Hello Aether', got '%s'", w.Body.String())
	}
}

func TestQueryParam(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/search", func(c *Context[testGlobal]) {
		q := c.Query("q")
		c.String(http.StatusOK, "search: %s", q)
	})

	req := httptest.NewRequest("GET", "/search?q=aether", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Body.String() != "search: aether" {
		t.Errorf("expected body 'search: aether', got '%s'", w.Body.String())
	}
}

func TestPathParam(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/users/{id}", func(c *Context[testGlobal]) {
		id := c.Param("id")
		c.String(http.StatusOK, "user: %s", id)
	})

	req := httptest.NewRequest("GET", "/users/42", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Body.String() != "user: 42" {
		t.Errorf("expected body 'user: 42', got '%s'", w.Body.String())
	}
}

func TestMiddleware(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	r.Use(func(c *Context[testGlobal]) {
		c.Res().Header().Set("X-Custom", "middleware-hit")
		c.Next()
	})

	Get(r, "/mw", func(c *Context[testGlobal]) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/mw", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("X-Custom") != "middleware-hit" {
		t.Errorf("expected X-Custom header 'middleware-hit', got '%s'", w.Header().Get("X-Custom"))
	}
}

func TestGroupRoutes(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	api := NewGroup("/api", r)
	Get(api, "/status", func(c *Context[testGlobal]) {
		c.String(http.StatusOK, "api ok")
	})

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "api ok" {
		t.Errorf("expected body 'api ok', got '%s'", w.Body.String())
	}
}

func TestGlobalAccess(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/global", func(c *Context[testGlobal]) {
		c.String(http.StatusOK, "%s", c.Global.AppName)
	})

	req := httptest.NewRequest("GET", "/global", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Body.String() != "TestApp" {
		t.Errorf("expected body 'TestApp', got '%s'", w.Body.String())
	}
}

func TestCookies(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/set-cookie", func(c *Context[testGlobal]) {
		c.SetCookie(&http.Cookie{
			Name:  "session",
			Value: "abc123",
			Path:  "/",
		})
		c.String(http.StatusOK, "cookie set")
	})

	Get(r, "/get-cookie", func(c *Context[testGlobal]) {
		cookie, err := c.Cookie("session")
		if err != nil {
			c.String(http.StatusBadRequest, "no cookie")
			return
		}
		c.String(http.StatusOK, "%s", cookie.Value)
	})

	// Test set cookie
	req := httptest.NewRequest("GET", "/set-cookie", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie to be set")
	}
	if cookies[0].Name != "session" || cookies[0].Value != "abc123" {
		t.Errorf("expected session=abc123, got %s=%s", cookies[0].Name, cookies[0].Value)
	}

	// Test get cookie
	req2 := httptest.NewRequest("GET", "/get-cookie", nil)
	req2.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Body.String() != "abc123" {
		t.Errorf("expected body 'abc123', got '%s'", w2.Body.String())
	}
}

func TestRenderWithoutTemplate(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/render", func(c *Context[testGlobal]) {
		err := c.Render(http.StatusOK, "index", nil)
		if err != nil {
			c.String(http.StatusInternalServerError, "%s", err.Error())
		}
	})

	req := httptest.NewRequest("GET", "/render", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestPutRoute(t *testing.T) {
	type updateBody struct {
		Name string `json:"name"`
	}

	app := newTestApp()
	r := app.Router()

	Put[testGlobal, updateBody](r, "/users/{id}", func(c *Context[testGlobal], body updateBody) {
		c.JSON(http.StatusOK, map[string]string{
			"id":   c.Param("id"),
			"name": body.Name,
		})
	})

	payload := `{"name":"Updated"}`
	req := httptest.NewRequest("PUT", "/users/1", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "1" || resp["name"] != "Updated" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestDeleteRoute(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Delete(r, "/users/{id}", func(c *Context[testGlobal]) {
		c.JSON(http.StatusOK, map[string]string{"deleted": c.Param("id")})
	})

	req := httptest.NewRequest("DELETE", "/users/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["deleted"] != "99" {
		t.Errorf("expected deleted '99', got '%s'", resp["deleted"])
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/panic", func(c *Context[testGlobal]) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestCacheAccessor(t *testing.T) {
	store, err := NewOtterStore(100)
	if err != nil {
		t.Fatalf("failed to create otter store: %v", err)
	}

	app := New[testGlobal](&Config[testGlobal]{
		Port:  0,
		Cache: store,
		Global: testGlobal{AppName: "CacheTest"},
	})
	r := app.Router()

	Get(r, "/cache-set", func(c *Context[testGlobal]) {
		err := c.Cache().Set(c.Req().Context(), "key1", "value1")
		if err != nil {
			c.String(http.StatusInternalServerError, "cache error")
			return
		}
		c.String(http.StatusOK, "cached")
	})

	Get(r, "/cache-get", func(c *Context[testGlobal]) {
		val, ok := c.Cache().Get(c.Req().Context(), "key1")
		if !ok {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.String(http.StatusOK, "%s", val.(string))
	})

	// Set cache
	req := httptest.NewRequest("GET", "/cache-set", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Get cache
	req2 := httptest.NewRequest("GET", "/cache-get", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Body.String() != "value1" {
		t.Errorf("expected 'value1', got '%s'", w2.Body.String())
	}
}

func TestWrapMiddleware(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	stdMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Wrapped", "true")
			next.ServeHTTP(w, req)
		})
	}

	r.Use(WrapMiddleware[testGlobal](stdMiddleware))

	Get(r, "/wrapped", func(c *Context[testGlobal]) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/wrapped", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Wrapped") != "true" {
		t.Errorf("expected X-Wrapped header 'true', got '%s'", w.Header().Get("X-Wrapped"))
	}
}
