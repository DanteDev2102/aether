package main

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// --- Modelos ---

type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type CreateItemBody struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

// --- Store en memoria ---

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

// --- Helpers ---

func writeJSON(c *Context, status int, data any) {
	c.JSON(status, data)
}

func writeError(c *Context, status int, msg string) {
	writeJSON(c, status, map[string]string{"error": msg})
}

func pathID(c *Context) (int, bool) {
	id, err := strconv.Atoi(c.req.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func main() {
	app := New(&Config{Host: "localhost", Port: 8080})
	r := app.Router()
	store := NewItemStore()

	app.AddCron("daily_cleanup", 2*time.Second, func(ctx context.Context, log Logger) {
		log.Info("Running scheduled daily cleanup... (mocking background task)")
	})

	Get(r, "/health", func(c *Context) {
		c.Log.Info("Health check was called")
		c.res.WriteHeader(200)
		c.res.Write([]byte("OK"))
	})

	Get(r, "/items", func(c *Context) {
		writeJSON(c, http.StatusOK, store.GetAll())
	})

	Get(r, "/items/{id}", func(c *Context) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		item, found := store.GetByID(id)
		if !found {
			writeError(c, http.StatusNotFound, "item no encontrado")
			return
		}
		writeJSON(c, http.StatusOK, item)
	})

	Post(r, "/items", func(c *Context, body CreateItemBody) {
		if body.Name == "" {
			c.Log.Warn("Attempted to create item without a name")
			writeError(c, http.StatusBadRequest, "el campo 'name' es requerido")
			return
		}
		item := store.Create(body.Name, body.Qty)
		c.Log.Infof("Item created id=%d name=%s", item.ID, item.Name)
		writeJSON(c, http.StatusCreated, item)
	})

	Put(r, "/items/{id}", func(c *Context, body CreateItemBody) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		item, found := store.Update(id, body.Name, body.Qty)
		if !found {
			writeError(c, http.StatusNotFound, "item no encontrado")
			return
		}
		writeJSON(c, http.StatusOK, item)
	})

	Delete(r, "/items/{id}", func(c *Context) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		if !store.Delete(id) {
			writeError(c, http.StatusNotFound, "item no encontrado")
			return
		}
		c.Log.Infof("Item deleted id=%d", id)
		c.res.WriteHeader(http.StatusNoContent)
	})

	app.Listen()
}
