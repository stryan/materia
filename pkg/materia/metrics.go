package materia

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"primamateria.systems/materia/pkg/actions"
)

type metrics struct {
	lastRunSuccess   prometheus.Gauge
	lastRollback     prometheus.Gauge
	lastRunTimestamp prometheus.Gauge
	lastRunDuration  prometheus.Gauge

	lastPlanSteps *prometheus.GaugeVec
}

// RunResult is for metrics generation only, at least for now
type RunResult struct {
	Start, End time.Time
	Success    bool
	RolledBack bool
	Plan       []actions.Action
}

func newMetrics(reg prometheus.Registerer) *metrics {
	f := promauto.With(reg)
	return &metrics{
		lastRunSuccess: f.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_success",
			Help: "Whether the last execute succeeded",
		}),
		lastRollback: f.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_rollback",
			Help: "Whether the last execute was the result of a rollback",
		}),
		lastRunTimestamp: f.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_timestamp",
			Help: "When the last execute started (in unix time)",
		}),
		lastRunDuration: f.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_duration",
			Help: "How long in seconds the last execute took",
		}),
		lastPlanSteps: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "last_run_plan_steps",
			Help: "How many of action type in last plan generated",
		}, []string{"action"}),
	}
}

func (m *Materia) SaveRun(r RunResult) {
	if r.Success {
		m.metrics.lastRunSuccess.Set(1)
	} else {
		m.metrics.lastRunSuccess.Set(0)
	}
	if r.RolledBack {
		m.metrics.lastRollback.Set(1)
	} else {
		m.metrics.lastRollback.Set(0)
	}
	m.metrics.lastRunTimestamp.Set(float64(r.Start.Unix()))
	m.metrics.lastRunDuration.Set(r.End.Sub(r.Start).Seconds())

	metricSteps := map[string]int{}
	for _, v := range r.Plan {
		metricSteps[v.Todo.String()] = metricSteps[v.Todo.String()] + 1
	}
	for _, a := range actions.AllActionTypes() {
		if val, ok := metricSteps[a.String()]; ok {
			m.metrics.lastPlanSteps.WithLabelValues(a.String()).Set(float64(val))
		} else {
			m.metrics.lastPlanSteps.WithLabelValues(a.String()).Set(0)
		}
	}
}
