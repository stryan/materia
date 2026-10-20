package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/plan"
)

type planOutput struct {
	Timestamp        time.Time `toml:"timestamp" json:"timestamp"`
	Plan             []string  `toml:"plan" json:"plan"`
	ChangedResources []change  `json:"changed_resources" toml:"changed_resources"`
}

type executionOutput struct {
	Timestamp time.Time `json:"timestamp" toml:"timestamp"`

	Plan *plan.Plan `json:"plan" toml:"plan"`

	StartRevision string `json:"start_revision" toml:"start_revision"`
	EndRevision   string `json:"end_revision" toml:"end_revision"`

	TotalSteps     int `json:"total_steps" toml:"total_steps"`
	CompletedSteps int `json:"completed_steps" toml:"completed_steps"`
}

type change struct {
	ResourceName string `json:"resource_name" toml:"resource_name"`
	Before       string `json:"before" toml:"before"`
	After        string `json:"after" toml:"after"`
}

func (m *Materia) SaveExecution(ctx context.Context, p *plan.Plan, report ExecutionReport) error {
	sourceReps := m.Source.Report(ctx)
	reports := make([]SourceSyncReport, 0, len(sourceReps))
	for k, v := range sourceReps {
		newRep := SourceSyncReport{
			Name:          k,
			StartRevision: v.OldRevision,
			EndRevision:   v.NewRevision,
		}
		reports = append(reports, newRep)
	}
	execOutput := executionOutput{
		Timestamp: report.Timestamp,

		Plan: p,

		TotalSteps:     report.TotalSteps,
		CompletedSteps: report.StepsCompleted,
	}

	found := false
	for _, v := range reports {
		if v.Name == "materia" {
			execOutput.StartRevision = v.StartRevision
			execOutput.EndRevision = v.EndRevision
			found = true
			break
		}
	}
	if !found {
		return errors.New("invalid execution source report, no materia source")
	}
	content := bytes.NewBuffer([]byte{})
	err := toml.NewEncoder(content).Encode(execOutput)
	if err != nil {
		return fmt.Errorf("failed to encode output to TOML: %w", err)
	}
	return m.saveOutput(ctx, "lastrun", content.Bytes())
}

func (m *Materia) SavePlan(ctx context.Context, p *plan.Plan) error {
	planOutput := planOutput{
		Timestamp: time.Now(),
		Plan:      p.PrettyLines(),
	}
	dmp := diffmatchpatch.New()
	for _, a := range p.Steps() {
		if a.Todo == actions.ActionUpdate {
			before := dmp.DiffText1(a.DiffContent)
			after := dmp.DiffText2(a.DiffContent)
			if a.Target.Kind == components.ResourceTypePodmanSecret {
				before = "[redacted]"
				after = "[redacted]"
			}
			planOutput.ChangedResources = append(planOutput.ChangedResources, change{
				ResourceName: a.Target.Path,
				Before:       before,
				After:        after,
			})
		}
	}

	content := bytes.NewBuffer([]byte{})
	err := toml.NewEncoder(content).Encode(planOutput)
	if err != nil {
		return fmt.Errorf("failed to encode output to TOML: %w", err)
	}
	return m.saveOutput(ctx, "lastplan", content.Bytes())
}

func (m *Materia) saveOutput(ctx context.Context, name string, content []byte) error {
	if err := m.rotateOutput(ctx, name, 0); err != nil {
		return err
	}
	return m.Output.Install(ctx, outputPath(name, 0), content)
}

func outputPath(name string, index int) string {
	if index == 0 {
		return name + ".toml"
	}
	return fmt.Sprintf("%s-%d.toml", name, index)
}

// rotateOutput frees up filename-index by renaming the existing file to filename-index+1
func (m *Materia) rotateOutput(ctx context.Context, name string, index int) error {
	path := outputPath(name, index)
	exists, err := m.Output.Exists(ctx, path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if index+1 >= m.OutputMax {
		return m.Output.Remove(ctx, path)
	}
	// make room in the next slot first, then move this file into it
	if err := m.rotateOutput(ctx, name, index+1); err != nil {
		return err
	}
	return m.Output.Rename(ctx, path, outputPath(name, index+1))
}
