package pool

import (
	"context"
)

type Options struct {
	NoWorkers  int
	PoolSize   int
	ResultSize int
	Ctx        context.Context
}
