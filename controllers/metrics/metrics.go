package metrics

import (
	s "github.com/kyma-project/compass-manager/controllers/status"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricState   = "cm_states"
	MetricActions = "cm_actions"

	LabelState  = "state"
	LabelName   = "kyma_name"
	LabelAction = "action"

	ActionRegister   = "register"
	ActionConfigure  = "configure"
	ActionUnregister = "unregister"
)

type Metrics struct {
	states  *prometheus.GaugeVec
	actions *prometheus.CounterVec
}

func NewMetrics() Metrics {
	m := Metrics{
		states: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricState,
			Help: "Indicates the Status.state for Compass Mappings",
		}, []string{LabelName, LabelState}),

		actions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: MetricActions,
			Help: "Number of <action> performed on Kymas",
		}, []string{LabelName, LabelAction}),
	}
	metrics.Registry.MustRegister(m.states, m.actions)
	return m
}

func (m Metrics) IncConfigure(kymaName string) {
	m.actions.With(prometheus.Labels{
		LabelName:   kymaName,
		LabelAction: ActionConfigure,
	}).Inc()
}

func (m Metrics) IncRegister(kymaName string) {
	m.actions.With(prometheus.Labels{
		LabelName:   kymaName,
		LabelAction: ActionRegister,
	}).Inc()
}

func (m Metrics) IncUnregister(kymaName string) {
	m.actions.With(prometheus.Labels{
		LabelName:   kymaName,
		LabelAction: ActionUnregister,
	}).Inc()
}

func (m Metrics) UpdateState(kymaName string, status s.Status) {
	if status == s.Empty {
		m.setModuleStateGauge(kymaName, "")
		return
	}
	state := s.StateText(status)
	m.setModuleStateGauge(kymaName, state)
}

func (m Metrics) setModuleStateGauge(kymaName, state string) {
	for _, s := range []string{s.ReadyState, s.FailedState, s.ProcessingState} {
		val := 0.0
		if s == state {
			val = 1
		}
		m.states.With(prometheus.Labels{
			LabelName:  kymaName,
			LabelState: s,
		}).Set(val)
	}
}
