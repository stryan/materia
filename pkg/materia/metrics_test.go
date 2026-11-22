package materia

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"primamateria.systems/materia/pkg/actions"
)

func TestMateria_SaveRun(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string // description of this test case
		input RunResult
		want  want
	}{
		{
			name: "nothing to do",
			input: RunResult{
				Start:      now.Add(-2 * time.Second),
				End:        now,
				Success:    true,
				RolledBack: false,
				Plan:       []actions.Action{},
			},
			want: want{
				Success:   true,
				Timestamp: now.Add(-2 * time.Second).Unix(),
				Duration:  2,
				Steps:     map[actions.ActionType]int{},
			},
		},
		{
			name: "simple plan",
			input: RunResult{
				Start:      now.Add(-2 * time.Second),
				End:        now,
				Success:    true,
				RolledBack: false,
				Plan: []actions.Action{
					{Todo: actions.ActionInstall},
					{Todo: actions.ActionInstall},
				},
			},
			want: want{
				Success:   true,
				Timestamp: now.Add(-2 * time.Second).Unix(),
				Duration:  2,
				Steps: map[actions.ActionType]int{
					actions.ActionInstall: 2,
				},
			},
		},
		{
			name: "rollback",
			input: RunResult{
				Start:      now.Add(-2 * time.Second),
				End:        now,
				Success:    true,
				RolledBack: true,
				Plan: []actions.Action{
					{Todo: actions.ActionInstall},
					{Todo: actions.ActionRemove},
					{Todo: actions.ActionRestart},
				},
			},
			want: want{
				Success:   true,
				Rollback:  true,
				Timestamp: now.Add(-2 * time.Second).Unix(),
				Duration:  2,
				Steps: map[actions.ActionType]int{
					actions.ActionInstall: 1,
					actions.ActionRemove:  1,
					actions.ActionRestart: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseReg := prometheus.NewRegistry()
			reg := prometheus.WrapRegistererWithPrefix("materia_", baseReg)

			m := &Materia{
				metricsReg: baseReg,
				metrics:    newMetrics(reg),
			}
			m.SaveRun(tt.input)
			if err := testutil.GatherAndCompare(m.metricsReg, tt.want.text()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type want struct {
	Success   bool
	Rollback  bool
	Timestamp int64
	Duration  float64
	Steps     map[actions.ActionType]int
}

func (w want) text() io.Reader {
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	var result strings.Builder
	fmt.Fprintf(&result, `
# HELP materia_last_run_duration How long in seconds the last execute took
# TYPE materia_last_run_duration gauge
materia_last_run_duration %v
# HELP materia_last_run_plan_steps How many of action type in last plan generated
# TYPE materia_last_run_plan_steps gauge
`, w.Duration)
	for _, a := range actions.AllActionTypes() {
		fmt.Fprintf(&result, "materia_last_run_plan_steps{action=%q} %d\n", a.String(), w.Steps[a])
	}
	fmt.Fprintf(&result, `# HELP materia_last_run_rollback Whether the last execute was the result of a rollback
# TYPE materia_last_run_rollback gauge
materia_last_run_rollback %d
# HELP materia_last_run_success Whether the last execute succeeded
# TYPE materia_last_run_success gauge
materia_last_run_success %d
# HELP materia_last_run_timestamp When the last execute started (in unix time)
# TYPE materia_last_run_timestamp gauge
materia_last_run_timestamp %d
`, b2i(w.Rollback), b2i(w.Success), w.Timestamp)
	return strings.NewReader(result.String())
}
