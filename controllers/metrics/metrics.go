package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	configureCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "configure_counter",
		Help: "Number of cluster configured",
	})
	registerCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "register_counter",
		Help: "Number of cluster registered",
	})
	unregisterCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "unregister_counter",
		Help: "Number of cluster unregistered",
	})
)

func init() {
	metrics.Registry.MustRegister(configureCounter, registerCounter, unregisterCounter)
}

func IncrementConfigureCounter() {
	configureCounter.Inc()
}

func IncrementRegisterCounter() {
	registerCounter.Inc()
}

func IncrementUnregisterCounter() {
	unregisterCounter.Inc()
}
