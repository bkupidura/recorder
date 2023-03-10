package pool

import (
	"context"
)

// Options contains configurable options for the Pool.
type Options struct {
	NoWorkers  int // Number of workers to spawn.
	PoolSize   int // Number of tasks which can be queued for latter execution if workers are busy.
	ResultSize int // Number of task results which will be queued.
	Ctx        context.Context
}
