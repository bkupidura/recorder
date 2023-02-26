package metric

import (
	"recorder/internal/pool"
)

type Options struct {
	WorkingPools map[string]*pool.Pool
}
