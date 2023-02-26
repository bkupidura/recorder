package api

import (
	"recorder/internal/pool"
)

var (
	HTTPPort = 8080
)

type Options struct {
	RecordingPath string
	WorkingPools  map[string]*pool.Pool
	AuthUsers     map[string]string
}
