package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recorder/internal/pool"

	"github.com/stretchr/testify/require"
)

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		inputPoolsOpts []*pool.Options
		expectedCode   int
	}{
		{
			inputPoolsOpts: []*pool.Options{
				{},
			},
			expectedCode: http.StatusServiceUnavailable,
		},
		{
			inputPoolsOpts: []*pool.Options{
				{},
				{},
				{},
			},
			expectedCode: http.StatusServiceUnavailable,
		},
		{
			inputPoolsOpts: []*pool.Options{
				{NoWorkers: 1},
				{NoWorkers: 1},
				{},
			},
			expectedCode: http.StatusServiceUnavailable,
		},
		{
			inputPoolsOpts: []*pool.Options{
				{NoWorkers: 1},
				{NoWorkers: 1},
				{NoWorkers: 1},
			},
			expectedCode: http.StatusOK,
		},
	}
	for _, test := range tests {
		workingPools := make(map[string]*pool.Pool)
		for idx, opts := range test.inputPoolsOpts {
			workingPools[fmt.Sprint(idx)] = pool.New(opts)
		}
		time.Sleep(10 * time.Millisecond)
		handler := healthHandler(workingPools)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		require.Equal(t, test.expectedCode, w.Code)
	}
}

func TestRecordHandler(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "outputDir", "/data")
	ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]interface{}{})
	ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]interface{}{})

	tests := []struct {
		inputRequest  map[string]interface{}
		inputPoolOpts *pool.Options
		expectedCode  int
		expectedError string
		expectedResp  map[string]interface{}
	}{
		{
			inputRequest:  map[string]interface{}{},
			inputPoolOpts: &pool.Options{},
			expectedCode:  http.StatusBadRequest,
			expectedError: "stream url is required",
		},
		{
			inputRequest:  map[string]interface{}{"stream": ""},
			expectedCode:  http.StatusBadRequest,
			inputPoolOpts: &pool.Options{},
			expectedError: "stream url is required",
		},
		{
			inputRequest:  map[string]interface{}{"stream": "TestRecordHandler"},
			inputPoolOpts: &pool.Options{},
			expectedCode:  http.StatusInternalServerError,
		},
		{
			inputRequest: map[string]interface{}{"stream": "TestRecordHandler"},
			inputPoolOpts: &pool.Options{
				NoWorkers:  0,
				PoolSize:   3,
				ResultSize: 3,
				Ctx:        ctx,
			},
			expectedCode: http.StatusOK,
			expectedResp: map[string]interface{}{"Stream": "TestRecordHandler", "Length": float64(5), "Burst": float64(1), "Prefix": "unknown", "CamName": "unknown"},
		},
		{
			inputRequest: map[string]interface{}{"stream": "TestRecordHandler", "cam_name": "test_cam_name", "prefix": "random_prefix", "length": 15, "burst": 3},
			inputPoolOpts: &pool.Options{
				NoWorkers:  0,
				PoolSize:   3,
				ResultSize: 3,
				Ctx:        ctx,
			},
			expectedCode: http.StatusOK,
			expectedResp: map[string]interface{}{"Stream": "TestRecordHandler", "CamName": "test_cam_name", "Prefix": "random_prefix", "Length": float64(15), "Burst": float64(3)},
		},
	}

	for _, test := range tests {
		p := pool.New(test.inputPoolOpts)
		handler := recordHandler(p)

		body, _ := json.Marshal(test.inputRequest)
		req := httptest.NewRequest(http.MethodPost, "/api/record", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler(w, req)

		require.Equal(t, test.expectedCode, w.Code)

		res := w.Result()
		defer res.Body.Close()
		resp := make(map[string]interface{})
		unmarshalBody(res.Body, &resp)
		if test.expectedResp != nil {
			require.Equal(t, test.expectedResp, resp)
		}

		if w.Code != http.StatusOK {
			if test.expectedError != "" {
				require.Equal(t, test.expectedError, resp["error"])
			}
		}
	}
}

func unmarshalBody(body io.Reader, destination interface{}) interface{} {
	b, err := io.ReadAll(body)
	if err != nil {
		panic("unable to read body")
	}
	err = json.Unmarshal(b, &destination)
	if err != nil {
		panic("unable to unmarshal body")
	}
	return destination
}
