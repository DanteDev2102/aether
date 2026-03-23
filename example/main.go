package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/DantDev2102/aether"
	"github.com/DantDev2102/aether/middlewares"
)

type AppState struct {
	Version string
	StartAt time.Time
}

type CreateUserBody struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateUserBody struct {
	Name string `json:"name"`
}

func main() {
	// Inicializar otter cache
	cache, err := aether.NewOtterStore(10000)
	if err != nil {
		panic(err)
	}

	app := aether.New(&aether.Config[AppState]{
		Port:    8080,
		Timeout: 30,
		Cache:   cache,
		Logger: aether.NewLogger(aether.LogConfig{
			Stdout: true,
		}),
		Global: AppState{
			Version: "1.0.0",
			StartAt: time.Now(),
		},
	})

	r := app.Router()

	// ─── Global Middlewares ───────────────────────────────────────────
	r.Use(middlewares.CORSMiddleware[AppState](middlewares.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}))
	r.Use(middlewares.HelmetMiddleware[AppState](middlewares.DefaultHelmetConfig()))
	r.Use(middlewares.RequestIDMiddleware[AppState]())

	// ─── Public Routes ───────────────────────────────────────────────
	aether.Get(r, "/", func(c *aether.Context[AppState]) {
		_ = c.JSON(http.StatusOK, map[string]any{
			"message": "Welcome to Aether!",
			"version": c.Global.Version,
			"uptime":  time.Since(c.Global.StartAt).String(),
		})
	})

	aether.Get(r, "/health", func(c *aether.Context[AppState]) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// ─── API Group ───────────────────────────────────────────────────
	api := aether.NewGroup("/api/v1", r)

	// Rate limit the API
	api.Use(middlewares.RateLimiterMiddleware[AppState](middlewares.RateLimiterConfig{
		Limit:  60,
		Window: time.Minute,
	}))

	// Users CRUD
	aether.Get(api, "/users", func(c *aether.Context[AppState]) {
		_ = c.JSON(http.StatusOK, []map[string]any{
			{"id": 1, "name": "Alice", "email": "alice@example.com"},
			{"id": 2, "name": "Bob", "email": "bob@example.com"},
		})
	})

	aether.Get(api, "/users/{id}", func(c *aether.Context[AppState]) {
		id := c.Param("id")
		_ = c.JSON(http.StatusOK, map[string]string{
			"id":   id,
			"name": "Alice",
		})
	})

	aether.Post[AppState, CreateUserBody](api, "/users", func(c *aether.Context[AppState], body CreateUserBody) {
		_ = c.JSON(http.StatusCreated, map[string]any{
			"id":    3,
			"name":  body.Name,
			"email": body.Email,
		})
	})

	aether.Put[AppState, UpdateUserBody](api, "/users/{id}", func(c *aether.Context[AppState], body UpdateUserBody) {
		_ = c.JSON(http.StatusOK, map[string]any{
			"id":   c.Param("id"),
			"name": body.Name,
		})
	})

	aether.Delete(api, "/users/{id}", func(c *aether.Context[AppState]) {
		_ = c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("User %s deleted", c.Param("id")),
		})
	})

	// ─── Cache Example ───────────────────────────────────────────────
	aether.Get(api, "/cached", func(c *aether.Context[AppState]) {
		ctx := c.Req().Context()
		val, ok := c.Cache().Get(ctx, "expensive_data")
		if ok {
			_ = c.JSON(http.StatusOK, map[string]any{
				"source": "cache",
				"data":   val,
			})
			return
		}

		data := map[string]string{"result": "computed value"}
		_ = c.Cache().Set(ctx, "expensive_data", data)

		_ = c.JSON(http.StatusOK, map[string]any{
			"source": "computed",
			"data":   data,
		})
	})

	// ─── Cookie Example ──────────────────────────────────────────────
	aether.Get(r, "/set-cookie", func(c *aether.Context[AppState]) {
		c.SetCookie(&http.Cookie{
			Name:     "theme",
			Value:    "dark",
			Path:     "/",
			MaxAge:   86400,
			HttpOnly: true,
		})
		_ = c.JSON(http.StatusOK, map[string]string{"message": "Cookie set!"})
	})

	aether.Get(r, "/get-cookie", func(c *aether.Context[AppState]) {
		cookie, err := c.Cookie("theme")
		if err != nil {
			_ = c.JSON(http.StatusNotFound, map[string]string{"error": "Cookie not found"})
			return
		}
		_ = c.JSON(http.StatusOK, map[string]string{"theme": cookie.Value})
	})

	// ─── Protected Routes (JWT) ──────────────────────────────────────
	protected := aether.NewGroup("/admin", r)
	protected.Use(middlewares.JWTMiddleware[AppState](middlewares.JWTConfig{
		Secret: []byte("my-super-secret-key"),
	}))

	aether.Get(protected, "/dashboard", func(c *aether.Context[AppState]) {
		claims := middlewares.GetMapClaims(c)
		_ = c.JSON(http.StatusOK, map[string]any{
			"message": "Welcome to admin dashboard",
			"claims":  claims,
		})
	})

	// ─── SSE Example ─────────────────────────────────────────────────
	aether.Get(r, "/events", func(c *aether.Context[AppState]) {
		rc, err := c.SSE()
		if err != nil {
			c.Log().Errorf("SSE error: %v", err)
			return
		}

		for i := 0; i < 5; i++ {
			_, _ = fmt.Fprintf(c.Res(), "data: Event %d at %s\n\n", i, time.Now().Format(time.RFC3339))
			_ = rc.Flush()
			time.Sleep(1 * time.Second)
		}
	})

	// ─── Static Files ────────────────────────────────────────────────
	aether.Static(r, "/static/", "./public")

	// ─── Cron Jobs ───────────────────────────────────────────────────
	app.AddCron("health-check", 30*time.Second, func(ctx context.Context, log aether.Logger) {
		log.Info("Health check: everything is fine ✓")
	})

	// ─── Start Server ────────────────────────────────────────────────
	_ = app.Listen()
}
