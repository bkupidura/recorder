package api

import (
	"fmt"
	"net/http"

	"recorder/internal/pool"
	"recorder/internal/task"

	"github.com/go-chi/render"
)

// errResponse describes error response for any API call.
type errResponse struct {
	Err            error  `json:"-"`               // low-level runtime error
	HTTPStatusCode int    `json:"-"`               // http response status code
	StatusText     string `json:"status"`          // user-level status message
	AppCode        int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText      string `json:"error,omitempty"` // application-level error message, for debugging
}

// Render response.
func (e *errResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// invalidRequestError returns 400 http error in case wrong requests parameters
// are sent to API endpoint.
func invalidRequestError(err error) render.Renderer {
	return &errResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Invalid request.",
		ErrorText:      err.Error(),
	}
}

// unableToPerformError returns 500 http error in case we are not able to
// perform request.
func unableToPerformError(err error) render.Renderer {
	return &errResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Unable to perform request.",
		ErrorText:      err.Error(),
	}
}

// healthHandler returns /healthz and /ready endpoint handler.
// It just check if every work pool have running workers.
func healthHandler(workingPools map[string]*pool.Pool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, pool := range workingPools {
			if !pool.Running() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

// apiRecordRequest describes API request used to start recording.
type apiRecordRequest struct {
	Stream  string `json:"stream"`
	CamName string `json:"cam_name"`
	Prefix  string `json:"prefix"`
	Length  int64  `json:"length"`
	Burst   int64  `json:"burst"`
}

// Bind validates request.
func (req *apiRecordRequest) Bind(r *http.Request) error {
	if req.Stream == "" {
		return fmt.Errorf("stream url is required")
	}
	if req.CamName == "" {
		req.CamName = "unknown"
	}
	if req.Prefix == "" {
		req.Prefix = "unknown"
	}
	if req.Length < 5 {
		req.Length = 5
	}
	if req.Burst < 1 {
		req.Burst = 1
	}
	return nil
}

// recorderHandler will start recording.
// apiRecordRequest should be passed.
func recordHandler(recordPool *pool.Pool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		request := &apiRecordRequest{}
		if err := render.Bind(r, request); err != nil {
			render.Render(w, r, invalidRequestError(err))
			return
		}
		tRecord := &task.Record{
			Stream:  request.Stream,
			Prefix:  request.Prefix,
			CamName: request.CamName,
			Length:  request.Length,
			Burst:   request.Burst,
		}

		if err := recordPool.Execute(tRecord.Do); err != nil {
			render.Render(w, r, unableToPerformError(err))
			return
		}
		render.JSON(w, r, tRecord)
	}
}
