package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
	"github.com/DanteDev2102/aether"
)

type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type CreateItemBody struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type ItemStore struct {
	mu      sync.RWMutex
	items   map[int]Item
	counter int
}

func NewItemStore() *ItemStore {
	return &ItemStore{items: make(map[int]Item)}
}

func (s *ItemStore) GetAll() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	return list
}

func (s *ItemStore) GetByID(id int) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

func (s *ItemStore) Create(name string, qty int) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	item := Item{ID: s.counter, Name: name, Qty: qty}
	s.items[s.counter] = item
	return item
}

func (s *ItemStore) Update(id int, name string, qty int) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return Item{}, false
	}
	item := Item{ID: id, Name: name, Qty: qty}
	s.items[id] = item
	return item, true
}

func (s *ItemStore) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return false
	}
	delete(s.items, id)
	return true
}

type GlobalState struct {
	Store *ItemStore
}

func writeJSON[T any](c *Context[T], status int, data any) {
	c.JSON(status, data)
}

func writeError[T any](c *Context[T], status int, msg string) {
	writeJSON(c, status, map[string]string{"error": msg})
}

func pathID[T any](c *Context[T]) (int, bool) {
	id, err := strconv.Atoi(c.req.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

type SessionData struct {
	UserID string
	Role   string
}

func main() {
	customLogger := aether.NewLogger(LogConfig{
		Stdout:    true,
		FilePaths: []string{"logs/app.log", "logs/debug/trace.log"},
	})

	store := NewItemStore()

	app := aether.New[GlobalState](&Config[GlobalState]{
		Host:    "localhost",
		Port:    8080,
		Logger:  customLogger,
		Global:  GlobalState{Store: store},
		Timeout: 1,
		ErrorHandler: func(c *Context[GlobalState], err any) {
			c.Log.Error("Enviando Notificacion a Discord/Slack por culpa del Panic!")
			c.JSON(http.StatusInternalServerError, map[string]any{
				"status": "fatal",
				"msg":    "Nuestros ingenieros han sido notificados",
				"reason": err,
			})
		},
	})
	r := app.Router()

	aether.Static(r, "/assets", "./public")

	app.AddCron("daily_cleanup", 2*time.Second, func(ctx context.Context, log Logger) {
		log.Info("Running scheduled daily cleanup... (mocking background task)")
	})

	aether.Get(r, "/health", func(c *Context[GlobalState]) {
		c.Log.Info("Health check was called")
		c.res.WriteHeader(200)
		c.res.Write([]byte("OK"))
	})

	aether.Get(r, "/panic", func(c *Context[GlobalState]) {
		c.Log.Info("Atencion: forzando un panic para probar el RecoveryMiddleware!")
		var ptr *int
		_ = *ptr
	})

	aether.Get(r, "/items", func(c *Context[GlobalState]) {
		writeJSON(c, http.StatusOK, c.Global.Store.GetAll())
	})

	aether.Get(r, "/items/{id}", func(c *Context[GlobalState]) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		item, found := c.Global.Store.GetByID(id)
		if !found {
			writeError(c, http.StatusNotFound, "item no encontrado")
			return
		}
		writeJSON(c, http.StatusOK, item)
	})

	aether.Post(r, "/items", func(c *Context[GlobalState], body CreateItemBody) {
		if body.Name == "" {
			c.Log.Warn("Attempted to create item without a name")
			writeError(c, http.StatusBadRequest, "el campo 'name' es requerido")
			return
		}
		item := c.Global.Store.Create(body.Name, body.Qty)
		c.Log.Infof("Item created id=%d name=%s", item.ID, item.Name)
		writeJSON(c, http.StatusCreated, item)
	})

	aether.Put(r, "/items/{id}", func(c *Context[GlobalState], body CreateItemBody) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		item, found := c.Global.Store.Update(id, body.Name, body.Qty)
		if !found {
			writeError(c, http.StatusNotFound, "item no encontrado")
			return
		}
		writeJSON(c, http.StatusOK, item)
	})

	aether.Delete(r, "/items/{id}", func(c *Context[GlobalState]) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		if !c.Global.Store.Delete(id) {
			writeError(c, http.StatusNotFound, "item no encontrado")
			return
		}
		c.Log.Infof("Item deleted id=%d", id)
		c.res.WriteHeader(http.StatusNoContent)
	})

	profileGroup := aether.NewGroup("/session", r)

	profileGroup.Use(func(c *Context[GlobalState]) {
		c.Log.Info("Autenticando usuario...")
		ctx := context.WithValue(c.ctx, "user_id", "12345-abcde")
		ctx = context.WithValue(ctx, "role", "admin")
		c.ctx = ctx
		c.Next()
	})

	aether.Get(profileGroup, "/profile", WithCustomContext(SessionData{}, func(c *CustomContext[GlobalState, SessionData]) {
		if uid, ok := c.ctx.Value("user_id").(string); ok {
			c.Data.UserID = uid
		}
		if role, ok := c.ctx.Value("role").(string); ok {
			c.Data.Role = role
		}

		c.Log.Infof("Bienvenido al perfil, %s (%s)", c.Data.UserID, c.Data.Role)
		c.JSON(http.StatusOK, map[string]any{
			"message": "Perfil cargado con CustomContext seguro",
			"session": c.Data,
			"global":  fmt.Sprintf("Tengo %d items guardados globales", len(c.Global.Store.GetAll())),
		})
	}))

	app.Listen()
}
