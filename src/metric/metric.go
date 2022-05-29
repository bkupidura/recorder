package metric

import (
	"github.com/asaskevich/EventBus"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricRecorderErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "recorder_errors_total",
			Help: "Recorder total errors",
		}, []string{"service"},
	)
	metricRecorderWorkers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "recorder_workers",
			Help: "Recorder running workers",
		}, []string{"service"},
	)
)

func recorderError(delta int, service string) {
	metricRecorderErrors.WithLabelValues(service).Add(float64(delta))
}

func recorderWorker(runningWorkers *int64, service string) {
	metricRecorderWorkers.WithLabelValues(service).Set(float64(*runningWorkers))
}

func Register(bus EventBus.Bus) error {
	prometheus.MustRegister(metricRecorderErrors)
	prometheus.MustRegister(metricRecorderWorkers)

	if err := bus.SubscribeAsync("metrics:recorder_error", recorderError, true); err != nil {
		return err
	}
	if err := bus.SubscribeAsync("metrics:recorder_worker", recorderWorker, true); err != nil {
		return err
	}

	return nil

}
