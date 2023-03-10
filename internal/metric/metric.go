package metric

import (
	"log"
	"time"

	"recorder/internal/pool"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	workingPoolErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "working_pool_errors_total",
		Help: "Total number of errors for working pool",
	}, []string{"pool"})
	workingPoolTaskInProgress = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "working_pool_task_in_progress",
		Help: "Number of currently running tasks",
	}, []string{"pool"})
	workingPoolWorkBacklog = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "working_pool_work_backlog",
		Help: "Number of tasks waiting in working pool",
	}, []string{"pool"})
)

func Initialize(opts *Options) {
	prometheus.MustRegister(workingPoolErrors)
	prometheus.MustRegister(workingPoolTaskInProgress)
	prometheus.MustRegister(workingPoolWorkBacklog)

	go collect(opts.WorkingPools)
}

func collect(workingPools map[string]*pool.Pool) {
	log.Printf("starting prometheus worker")
	for {
		for poolName, pool := range workingPools {
			workingPoolErrors.WithLabelValues(poolName).Set(float64(pool.Errors()))
			workingPoolTaskInProgress.WithLabelValues(poolName).Set(float64(pool.InProgress()))
			workingPoolWorkBacklog.WithLabelValues(poolName).Set(float64(pool.WorkBacklog()))
		}

		time.Sleep(5 * time.Second)
	}
}
