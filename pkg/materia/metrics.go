package materia

import "github.com/prometheus/client_golang/prometheus"

// # HELP materia_last_run_success Whether the last reconciliation run succeeded (1 = success, 0 = failure).
// # TYPE materia_last_run_success gauge
// materia_last_run_success 1
//
// # HELP materia_last_run_rollback Whether the last run triggered a rollback (1 = yes, 0 = no).
// # TYPE materia_last_run_rollback gauge
// materia_last_run_rollback 0
//
// # HELP materia_last_run_timestamp_seconds Unix epoch timestamp of the last run completion.
// # TYPE materia_last_run_timestamp_seconds gauge
// materia_last_run_timestamp_seconds 1776981600
//
// # HELP materia_last_run_duration_seconds Duration of the last reconciliation run in seconds.
// # TYPE materia_last_run_duration_seconds gauge
// materia_last_run_duration_seconds 3.42
//
// # HELP materia_last_run_info Metadata about the last applied state.
// # TYPE materia_last_run_info gauge
// materia_last_run_info{revision="a1b2c3d"} 1
//
// # HELP materia_last_run_plan_steps Number of steps executed in the last plan by action type.
// # TYPE materia_last_run_plan_steps gauge
// materia_last_run_plan_steps{action="create"} 0
// materia_last_run_plan_steps{action="update"} 1
// materia_last_run_plan_steps{action="remove"} 0
// materia_last_run_plan_steps{action="restart"} 1

type metrics struct {
	lastRun          prometheus.Gauge
	lastRollback     prometheus.Gauge
	lastRunTimestamp prometheus.Gauge
	lastRunDuration  prometheus.Gauge

	lastPlanSteps *prometheus.GaugeVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		lastRun: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_success",
			Help: "Whether the last execute succeeded",
		}),
		lastRollback: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_rollback",
			Help: "Whether the last execute was the result of a rollback",
		}),
		lastRunTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_timestamp",
			Help: "When the last execute occured (in unix time)",
		}),
		lastRunDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "last_run_duration",
			Help: "How long in seconds the last execute took",
		}),
		lastPlanSteps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "last_run_plan_steps",
			Help: "How many of action type in last plan generated",
		}, []string{"action"}),
	}
	reg.MustRegister(m.lastRun)
	reg.MustRegister(m.lastRollback)
	reg.MustRegister(m.lastRunTimestamp)
	reg.MustRegister(m.lastRunDuration)
	reg.MustRegister(m.lastPlanSteps)
	return m
}
