package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"recorder/internal/pool"

	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	tests := []struct {
		inputMethod  string
		inputPath    string
		inputAuth    [2]string
		expectedCode int
		auth         map[string]string
	}{
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/ready",
			expectedCode: http.StatusOK,
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/ready",
			expectedCode: http.StatusOK,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/healthz",
			expectedCode: http.StatusOK,
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/healthz",
			expectedCode: http.StatusOK,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/metrics",
			expectedCode: http.StatusOK,
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/metrics",
			expectedCode: http.StatusOK,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/debug/pprof/heap",
			expectedCode: http.StatusOK,
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/debug/pprof/heap",
			expectedCode: http.StatusUnauthorized,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/",
			expectedCode: http.StatusMovedPermanently,
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/",
			expectedCode: http.StatusUnauthorized,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/recordings/",
			expectedCode: http.StatusOK,
		},
		{
			inputMethod:  http.MethodGet,
			inputPath:    "/recordings/",
			expectedCode: http.StatusUnauthorized,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodPost,
			inputPath:    "/api/record",
			expectedCode: http.StatusBadRequest,
		},
		{
			inputMethod:  http.MethodPost,
			inputPath:    "/api/record",
			expectedCode: http.StatusUnauthorized,
			auth:         map[string]string{"test": "test"},
		},
		{
			inputMethod:  http.MethodPost,
			inputPath:    "/api/record",
			inputAuth:    [2]string{"test", "test"},
			expectedCode: http.StatusBadRequest,
			auth:         map[string]string{"test": "test"},
		},
	}

	workingPools := map[string]*pool.Pool{
		"record": pool.New(&pool.Options{
			NoWorkers: 3,
		}),
	}

	for _, test := range tests {
		body, _ := json.Marshal(map[string]interface{}{})

		req, err := http.NewRequest(test.inputMethod, test.inputPath, bytes.NewReader(body))
		require.Nil(t, err)

		req.Header.Set("Content-Type", "application/json")
		if len(test.inputAuth) > 0 {
			req.SetBasicAuth(test.inputAuth[0], test.inputAuth[1])
		}
		w := httptest.NewRecorder()

		router := NewRouter(&Options{
			AuthUsers:    test.auth,
			WorkingPools: workingPools,
		})

		httpServer := &http.Server{Addr: fmt.Sprintf(":%d", HTTPPort), Handler: router}
		go httpServer.ListenAndServe()
		defer httpServer.Shutdown(context.TODO())

		router.ServeHTTP(w, req)

		require.Equal(t, test.expectedCode, w.Code)
	}
}
