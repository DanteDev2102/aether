package aether

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testGlobal struct {
	AppName string
}

func newTestApp() *App[testGlobal] {
	return New[testGlobal](&Config[testGlobal]{
		Port:   0,
		Global: testGlobal{AppName: "TestApp"},
	})
}

func TestGetRoute(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/hello", func(c *Context[testGlobal]) {
		_ = c.String(http.StatusOK, "Hello, World!")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/hello", nil)
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
		_ = c.JSON(http.StatusCreated, map[string]string{"name": body.Name})
	})

	payload := `{"name":"Aether"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/users", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "Aether" {
		t.Errorf("expected name 'Aether', got '%s'", resp["name"])
	}
}

func TestContextJSON(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/json", func(c *Context[testGlobal]) {
		_ = c.JSON(http.StatusOK, map[string]string{"key": "value"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/json", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["key"] != "value" {
		t.Errorf("expected key 'value', got '%s'", resp["key"])
	}
}

func TestContextString(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Get(r, "/text", func(c *Context[testGlobal]) {
		_ = c.Stringf(http.StatusOK, "Hello %s", "Aether")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/text", nil)
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
		_ = c.Stringf(http.StatusOK, "search: %s", q)
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/search?q=aether", nil)
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
		_ = c.Stringf(http.StatusOK, "user: %s", id)
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/users/42", nil)
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
		_ = c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/mw", nil)
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
		_ = c.String(http.StatusOK, "api ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/status", nil)
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
		_ = c.Stringf(http.StatusOK, "%s", c.Global.AppName)
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/global", nil)
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
		_ = c.String(http.StatusOK, "cookie set")
	})

	Get(r, "/get-cookie", func(c *Context[testGlobal]) {
		cookie, err := c.Cookie("session")
		if err != nil {
			_ = c.String(http.StatusBadRequest, "no cookie")
			return
		}
		_ = c.Stringf(http.StatusOK, "%s", cookie.Value)
	})

	// Test set cookie
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/set-cookie", nil)
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
	req2 := httptest.NewRequestWithContext(context.Background(), "GET", "/get-cookie", nil)
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
			_ = c.Stringf(http.StatusInternalServerError, "%s", err.Error())
		}
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/render", nil)
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
		_ = c.JSON(http.StatusOK, map[string]string{
			"id":   c.Param("id"),
			"name": body.Name,
		})
	})

	payload := `{"name":"Updated"}`
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/users/1", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "1" || resp["name"] != "Updated" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestDeleteRoute(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	Delete(r, "/users/{id}", func(c *Context[testGlobal]) {
		_ = c.JSON(http.StatusOK, map[string]string{"deleted": c.Param("id")})
	})

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/users/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
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

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestCacheAccessor(t *testing.T) {
	store, err := NewOtterStore(OtterConfig{MaxSize: 100})
	if err != nil {
		t.Fatalf("failed to create otter store: %v", err)
	}

	app := New[testGlobal](&Config[testGlobal]{
		Port:   0,
		Cache:  store,
		Global: testGlobal{AppName: "CacheTest"},
	})
	r := app.Router()

	Get(r, "/cache-set", func(c *Context[testGlobal]) {
		err := c.Cache().Set(c.Req().Context(), "key1", "value1")
		if err != nil {
			_ = c.String(http.StatusInternalServerError, "cache error")
			return
		}
		_ = c.String(http.StatusOK, "cached")
	})

	Get(r, "/cache-get", func(c *Context[testGlobal]) {
		val, ok := c.Cache().Get(c.Req().Context(), "key1")
		if !ok {
			_ = c.String(http.StatusNotFound, "not found")
			return
		}
		_ = c.Stringf(http.StatusOK, "%s", val.(string))
	})

	// Set cache
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/cache-set", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Get cache
	req2 := httptest.NewRequestWithContext(context.Background(), "GET", "/cache-get", nil)
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
		_ = c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/wrapped", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Wrapped") != "true" {
		t.Errorf("expected X-Wrapped header 'true', got '%s'", w.Header().Get("X-Wrapped"))
	}
}

func TestGlobalMiddleware_RunsOnUnmatchedRoutes(t *testing.T) {
	app := newTestApp()
	r := app.Router()

	r.Use(func(c *Context[testGlobal]) {
		c.Res().Header().Set("X-Global", "1")
		if c.Req().Method == http.MethodOptions {
			c.Res().WriteHeader(http.StatusNoContent)
			return
		}
		c.Next()
	})

	Post(r, "/items", func(c *Context[testGlobal], _ struct{}) {
		_ = c.String(http.StatusOK, "ok")
	})

	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodGet, "/items", http.StatusMethodNotAllowed},
		{http.MethodOptions, "/items", http.StatusNoContent},
	}

	for _, tc := range cases {
		req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != tc.want {
			t.Errorf("%s %s: expected status %d, got %d", tc.method, tc.path, tc.want, w.Code)
		}
		if w.Header().Get("X-Global") != "1" {
			t.Errorf("%s %s: global middleware did not run", tc.method, tc.path)
		}
	}
}

func TestGroupRoutes_ConcurrentRequests(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	api := NewGroup("/api", r)
	Get(api, "/ping", func(c *Context[testGlobal]) {
		_ = c.String(http.StatusOK, "pong")
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/ping", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			}
		}()
	}
	wg.Wait()
}

func TestContextString_IsLiteral(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Get(r, "/pct", func(c *Context[testGlobal]) {
		_ = c.String(http.StatusOK, "100%")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/pct", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "100%" {
		t.Errorf("expected body '100%%', got '%s'", w.Body.String())
	}
}

func TestPostRoute_EmptyBody(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Post(r, "/logout", func(c *Context[testGlobal], _ struct{}) {
		_ = c.String(http.StatusOK, "bye")
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestPostRoute_InvalidBody(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Post(r, "/users", func(c *Context[testGlobal], _ struct{ Name string }) {
		_ = c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/users", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected JSON error, got Content-Type %q", ct)
	}
}

func TestPostRoute_BodyTooLarge(t *testing.T) {
	app := New(&Config[testGlobal]{MaxBodyBytes: 8})
	r := app.Router()
	Post(r, "/users", func(c *Context[testGlobal], _ struct{ Name string }) {
		_ = c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/users", bytes.NewBufferString(`{"Name":"a very long name"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}

func TestContextJSON_EncodeErrorWritesNothing(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	var encErr error
	Get(r, "/bad", func(c *Context[testGlobal]) {
		encErr = c.JSON(http.StatusOK, map[string]any{"ch": make(chan int)})
		if encErr != nil {
			_ = c.String(http.StatusInternalServerError, "encode failed")
		}
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if encErr == nil {
		t.Fatalf("expected an encoding error")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestRecoveryMiddleware_DoesNotLeakPanic(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Get(r, "/boom", func(_ *Context[testGlobal]) {
		panic("db password=secret")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/boom", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("secret")) {
		t.Errorf("panic value leaked to client: %s", w.Body.String())
	}
}

func TestRecoveryMiddleware_AfterPartialWrite(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Get(r, "/half", func(c *Context[testGlobal]) {
		_ = c.String(http.StatusOK, "partial")
		panic("late failure")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/half", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "partial" {
		t.Errorf("expected body to stay 'partial', got '%s'", w.Body.String())
	}
}

func newQuietApp() *App[testGlobal] {
	return New(&Config[testGlobal]{
		Host:            "127.0.0.1",
		ShutdownTimeout: time.Second,
		Logger:          NewLogger(LogConfig{}),
	})
}

func TestServe_ShutsDownOnContextCancel(t *testing.T) {
	app := newQuietApp()
	Get(app.Router(), "/ping", func(c *Context[testGlobal]) {
		_ = c.String(http.StatusOK, "pong")
	})

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var cronRuns atomic.Int32
	app.AddCron("tick", time.Hour, func(_ context.Context, _ Logger) { cronRuns.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Serve(ctx, ln) }()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+ln.Addr().String()+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil error on graceful shutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Serve did not return after context cancel")
	}
	if cronRuns.Load() != 1 {
		t.Errorf("expected cron job to run once on start, ran %d times", cronRuns.Load())
	}

	if err := app.Serve(context.Background(), ln); err == nil {
		t.Errorf("expected error when serving the same app twice")
	}
}

func TestListen_ReturnsErrorWhenPortBusy(t *testing.T) {
	var lc net.ListenConfig
	busy, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = busy.Close() }()

	app := newQuietApp()
	app.config.Port = busy.Addr().(*net.TCPAddr).Port

	if err := app.Listen(context.Background()); err == nil {
		t.Errorf("expected an error when the port is already in use")
	}
}

func TestRequestTimeout_SetsContextDeadline(t *testing.T) {
	app := New(&Config[testGlobal]{Timeout: 50 * time.Millisecond})
	Get(app.Router(), "/slow", func(c *Context[testGlobal]) {
		select {
		case <-c.Req().Context().Done():
			_ = c.String(http.StatusGatewayTimeout, "timeout")
		case <-time.After(time.Second):
			_ = c.String(http.StatusOK, "late")
		}
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/slow", nil)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status %d, got %d", http.StatusGatewayTimeout, w.Code)
	}
}

func TestOtterStore_TTL(t *testing.T) {
	ctx := context.Background()
	store, err := NewOtterStore(OtterConfig{MaxSize: 10, TTL: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_ = store.Set(ctx, "k", "v")
	if v, ok := store.Get(ctx, "k"); !ok || v != "v" {
		t.Fatalf("expected value before expiry, got %v %v", v, ok)
	}

	time.Sleep(150 * time.Millisecond)
	if _, ok := store.Get(ctx, "k"); ok {
		t.Errorf("expected entry to expire after TTL")
	}
}

func TestOtterStore_DeleteAndClear(t *testing.T) {
	ctx := context.Background()
	store, err := NewOtterStore(OtterConfig{TTL: -1})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_ = store.Set(ctx, "a", 1)
	_ = store.Set(ctx, "b", 2)
	_ = store.Delete(ctx, "a")
	if _, ok := store.Get(ctx, "a"); ok {
		t.Errorf("expected 'a' to be deleted")
	}
	_ = store.Clear(ctx)
	if _, ok := store.Get(ctx, "b"); ok {
		t.Errorf("expected cache to be cleared")
	}
}

func TestNewSlogLogger_WritesThroughHandler(t *testing.T) {
	var buf bytes.Buffer
	log := NewSlogLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	log.Infof("hello %s", "aether")
	log.Debug("dbg")
	log.Sync()

	out := buf.String()
	if !strings.Contains(out, "hello aether") || !strings.Contains(out, "level=DEBUG") {
		t.Errorf("unexpected log output: %q", out)
	}

	if NewSlogLogger(nil) == nil {
		t.Errorf("expected a logger for nil input")
	}
}

func TestRouterUse_PanicsAfterRoutes(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Get(r, "/x", func(c *Context[testGlobal]) {})

	defer func() {
		if recover() == nil {
			t.Errorf("expected Use after registering a route to panic")
		}
	}()
	r.Use(func(c *Context[testGlobal]) { c.Next() })
}

func TestRouterUse_PanicsAfterCreatingGroup(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	api := NewGroup("/api", r)
	api.Use(func(c *Context[testGlobal]) { c.Next() }) // the group itself is still open

	defer func() {
		if recover() == nil {
			t.Errorf("expected Use on parent after creating a group to panic")
		}
	}()
	r.Use(func(c *Context[testGlobal]) { c.Next() })
}

func TestGlobal_IsSharedPointer(t *testing.T) {
	app := New(&Config[testGlobal]{Global: testGlobal{AppName: "orig"}})
	r := app.Router()
	Get(r, "/set", func(c *Context[testGlobal]) {
		c.Global.AppName = "changed"
	})
	Get(r, "/get", func(c *Context[testGlobal]) {
		_ = c.String(http.StatusOK, c.Global.AppName)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), "GET", "/set", nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "GET", "/get", nil))

	if w.Body.String() != "changed" {
		t.Errorf("expected Global mutation to persist, got %q", w.Body.String())
	}
}

func TestContextCopy_UsableAfterRequest(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	got := make(chan string, 1)
	Get(r, "/users/{id}", func(c *Context[testGlobal]) {
		cp := c.Copy()
		go func() {
			time.Sleep(20 * time.Millisecond) // the request has finished by now
			if cp.Req().Context().Err() != nil {
				got <- "context canceled"
				return
			}
			if _, err := cp.Res().Write([]byte("x")); !errors.Is(err, ErrDetachedContext) {
				got <- "write did not fail"
				return
			}
			got <- cp.Param("id") + " " + cp.Global.AppName
		}()
		_ = c.String(http.StatusOK, "accepted")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", srv.URL+"/users/7", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case v := <-got:
		if v != "7 TestApp" {
			t.Errorf("expected '7 TestApp' from copied context, got %q", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("background goroutine did not finish")
	}
}

func TestTimeout_DoesNotCutResponse(t *testing.T) {
	app := New(&Config[testGlobal]{Timeout: 30 * time.Millisecond, Logger: NewLogger(LogConfig{})})
	Get(app.Router(), "/slow", func(c *Context[testGlobal]) {
		time.Sleep(80 * time.Millisecond) // ignores the deadline on purpose
		_ = c.String(http.StatusOK, "finished")
	})

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = app.Serve(ctx, ln) }()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://"+ln.Addr().String()+"/slow", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request was cut: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "finished" {
		t.Errorf("expected full response, got %q", body)
	}
}

func TestSSE_ClearsTimeoutAndTracksStatus(t *testing.T) {
	app := New(&Config[testGlobal]{Timeout: 30 * time.Millisecond})
	var ctxErr error
	var status int
	Get(app.Router(), "/events", func(c *Context[testGlobal]) {
		rc, err := c.SSE()
		if err != nil {
			t.Errorf("SSE: %v", err)
			return
		}
		for i := 0; i < 3; i++ {
			_, _ = io.WriteString(c.Res(), "data: tick\n\n")
			_ = rc.Flush()
			time.Sleep(25 * time.Millisecond)
		}
		ctxErr = c.Req().Context().Err()
		status = c.Res().(ResponseWriter).Status()
	})

	srv := httptest.NewServer(app.Router())
	defer srv.Close()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if ctxErr != nil {
		t.Errorf("expected SSE to clear the request timeout, context error: %v", ctxErr)
	}
	if status != http.StatusOK || strings.Count(string(body), "tick") != 3 {
		t.Errorf("expected status 200 and 3 events, got %d and %q", status, body)
	}
}

func TestTimeout_ClientDisconnectStillCancels(t *testing.T) {
	app := New(&Config[testGlobal]{Timeout: time.Minute})
	canceled := make(chan struct{})
	Get(app.Router(), "/wait", func(c *Context[testGlobal]) {
		c.ClearTimeout()
		<-c.Req().Context().Done()
		close(canceled)
	})

	srv := httptest.NewServer(app.Router())
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/wait", nil)
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	if resp, err := http.DefaultClient.Do(req); err == nil {
		_ = resp.Body.Close()
	}

	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatalf("client disconnect did not cancel the request context")
	}
}

type upperWriter struct{ http.ResponseWriter }

func (u *upperWriter) Write(b []byte) (int, error) { return u.ResponseWriter.Write(bytes.ToUpper(b)) }

func TestWrapMiddleware_HonorsReplacedWriter(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	var status int
	r.Use(func(c *Context[testGlobal]) {
		c.Next()
		status = c.Res().(ResponseWriter).Status()
	})
	r.Use(WrapMiddleware[testGlobal](func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(&upperWriter{w}, req)
		})
	}))
	Get(r, "/w", func(c *Context[testGlobal]) { _ = c.String(http.StatusCreated, "abc") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "GET", "/w", nil))

	if w.Body.String() != "ABC" {
		t.Errorf("expected body written through the wrapped writer, got %q", w.Body.String())
	}
	if status != http.StatusCreated {
		t.Errorf("expected outer middleware to see status %d, got %d", http.StatusCreated, status)
	}
}

func TestFormBind_InvalidValueAndTags(t *testing.T) {
	type form struct {
		Name string   `json:"name,omitempty"`
		Age  int      `form:"age"`
		Tags []string `form:"tag"`
		IDs  []int    `form:"id"`
		Skip string   `json:"-"`
	}
	app := newTestApp()
	r := app.Router()
	Post(r, "/f", func(c *Context[testGlobal], f form) { _ = c.JSON(http.StatusOK, f) })

	send := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(context.Background(), "POST", "/f", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := send("name=ana&age=30&tag=a&tag=b&id=1&id=2&Skip=x")
	want := `{"name":"ana","Age":30,"Tags":["a","b"],"IDs":[1,2]}`
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("expected %s, got %d %s", want, w.Code, w.Body.String())
	}

	for _, bad := range []string{"age=abc", "id=1&id=x", "age=99999999999999999999"} {
		if w := send(bad); w.Code != http.StatusBadRequest {
			t.Errorf("%q: expected status %d, got %d", bad, http.StatusBadRequest, w.Code)
		}
	}
}

func TestRouter_UnmatchedRoutes(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Get(r, "/items", func(c *Context[testGlobal]) { _ = c.String(http.StatusOK, "list") })
	Post(r, "/items", func(c *Context[testGlobal], _ struct{}) { _ = c.String(http.StatusCreated, "created") })
	Get(r, "/dir/", func(c *Context[testGlobal]) { _ = c.String(http.StatusOK, "dir") })

	cases := []struct {
		method, path string
		want         int
		allow        string
		location     string
	}{
		{http.MethodDelete, "/items", http.StatusMethodNotAllowed, "GET, HEAD, POST", ""},
		{http.MethodHead, "/items", http.StatusOK, "", ""},
		{http.MethodGet, "/nope", http.StatusNotFound, "", ""},
		{http.MethodGet, "/dir", http.StatusMovedPermanently, "", "/dir/"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("%s %s: expected status %d, got %d", tc.method, tc.path, tc.want, w.Code)
		}
		if got := w.Header().Get("Allow"); got != tc.allow {
			t.Errorf("%s %s: expected Allow %q, got %q", tc.method, tc.path, tc.allow, got)
		}
		if got := w.Header().Get("Location"); got != tc.location {
			t.Errorf("%s %s: expected Location %q, got %q", tc.method, tc.path, tc.location, got)
		}
	}
}

func TestStatic_AtRootCoexistsWithFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("static"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	app := newTestApp()
	r := app.Router()
	Static(r, "/", dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "GET", "/a.txt", nil))
	if w.Code != http.StatusOK || w.Body.String() != "static" {
		t.Errorf("expected static file, got %d %q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "POST", "/a.txt", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestBind_GetBodyBeforeBind(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	type user struct{ Name string }
	Get(r, "/raw", func(c *Context[testGlobal]) {
		raw, err := c.GetBody()
		if err != nil {
			t.Errorf("GetBody: %v", err)
		}
		var u user
		if err := c.Bind(&u); err != nil {
			t.Errorf("Bind after GetBody: %v", err)
		}
		_ = c.Stringf(http.StatusOK, "%s|%s", raw, u.Name)
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/raw", strings.NewReader(`{"Name":"ana"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != `{"Name":"ana"}|ana` {
		t.Errorf("unexpected body %q", w.Body.String())
	}
}

func TestPostRoute_TruncatedJSON(t *testing.T) {
	app := newTestApp()
	r := app.Router()
	Post(r, "/u", func(c *Context[testGlobal], _ struct{ Name string }) { _ = c.String(http.StatusOK, "ok") })

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/u", strings.NewReader(`{"Name":"an`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
