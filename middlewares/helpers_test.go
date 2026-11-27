package middlewares

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/DanteDev2102/aether"
)

type testGlobal struct{}

type testCtx = aether.Context[testGlobal]

func newTestRouter(mws ...aether.HandlerFunc[testGlobal]) *aether.Router[testGlobal] {
	app := aether.New(&aether.Config[testGlobal]{
		Logger: aether.NewLogger(aether.LogConfig{}),
	})
	r := app.Router()
	r.Use(mws...)
	return r
}

func do(r http.Handler, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
