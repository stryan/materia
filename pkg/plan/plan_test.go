package plan

import (
	"fmt"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
)

var compRegistry = map[string]*components.Component{}

func clearRegistry() {
	for k := range compRegistry {
		delete(compRegistry, k)
	}
	compRegistry["root"] = mustComponent("root")
}

func reload() actions.Action {
	c, ok := compRegistry["root"]
	if !ok {
		c = mustComponent("root")
		c.State = components.StateRoot
		compRegistry["root"] = c
	}
	return actions.Action{
		Parent: c,
		Todo:   actions.ActionReload,
		Target: components.Resource{
			Kind: components.ResourceTypeHost,
		},
	}
}

func act(compname string, todo actions.ActionType, resName string, prio int) actions.Action {
	c, ok := compRegistry[compname]
	if !ok {
		c = mustComponent(compname)
		compRegistry[compname] = c
	}
	var res components.Resource
	if resName != "" {
		res = components.Resource{
			Path:   resName,
			Kind:   components.FindResourceType(resName),
			Parent: compname,
		}
	} else {
		res = components.Resource{
			Path:   compname,
			Kind:   components.ResourceTypeComponent,
			Parent: compname,
		}
	}
	result := actions.Action{
		Todo:     todo,
		Parent:   c,
		Target:   res,
		Priority: prio,
	}

	// fake a diff for updates
	if res.IsFile() && todo == actions.ActionUpdate {
		result.DiffContent = diffmatchpatch.New().DiffMain("hello", "goodbye", false)
	}
	return result
}

func Test_Plan(t *testing.T) {
	tests := []struct {
		name   string
		input  []actions.Action
		output []actions.Action
	}{
		{
			name: "basic install",
			input: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
			},
			output: []actions.Action{
				act("hello", actions.ActionInstall, "", 2),
				act("hello", actions.ActionInstall, "hello.container", 3),
			},
		},
		{
			name: "full install",
			input: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
				act("hello", actions.ActionInstall, "data.volume", 0),
				act("hello", actions.ActionStart, "hello.container", 0),
				reload(),
			},
			output: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
				act("hello", actions.ActionInstall, "data.volume", 0),
				reload(),
				act("hello", actions.ActionStart, "hello.container", 0),
			},
		},
		{
			name: "install multiple resources",
			input: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "data.volume", 0),
				act("hello", actions.ActionInstall, "app.network", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
			},
			output: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "data.volume", 0),
				act("hello", actions.ActionInstall, "app.network", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
			},
		},
		{
			name: "install multiple components",
			input: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
				act("goodbye", actions.ActionInstall, "", 0),
				act("goodbye", actions.ActionInstall, "goodbye.container", 0),
			},
			output: []actions.Action{
				act("goodbye", actions.ActionInstall, "", 0),
				act("goodbye", actions.ActionInstall, "goodbye.container", 0),
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.container", 0),
			},
		},
		{
			name: "remove with stop",
			input: []actions.Action{
				act("hello", actions.ActionStop, "hello.container", 0),
				act("hello", actions.ActionRemove, "hello.container", 0),
				act("hello", actions.ActionRemove, "", 0),
				reload(),
			},
			output: []actions.Action{
				act("hello", actions.ActionStop, "hello.container", 0),
				act("hello", actions.ActionRemove, "hello.container", 0),
				act("hello", actions.ActionRemove, "", 0),
				reload(),
			},
		},
		{
			name: "update with restart",
			input: []actions.Action{
				act("hello", actions.ActionUpdate, "hello.container", 0),
				act("hello", actions.ActionRestart, "hello.container", 0),
				reload(),
			},
			output: []actions.Action{
				act("hello", actions.ActionUpdate, "hello.container", 0),
				reload(),
				act("hello", actions.ActionRestart, "hello.container", 0),
			},
		},
		{
			name: "removal with stop and reload",
			input: []actions.Action{
				act("hello", actions.ActionStop, "hello.container", 0),
				reload(),
				act("hello", actions.ActionRemove, "hello.container", 0),
				act("hello", actions.ActionRemove, "", 0),
			},
			output: []actions.Action{
				act("hello", actions.ActionStop, "hello.container", 0),
				act("hello", actions.ActionRemove, "hello.container", 0),
				act("hello", actions.ActionRemove, "", 0),
				reload(),
			},
		},
		{
			name: "remove multiple resources with reload",
			input: []actions.Action{
				act("hello", actions.ActionStop, "hello.container", 0),
				reload(),
				act("hello", actions.ActionRemove, "hello.container", 0),
				act("hello", actions.ActionRemove, "data.volume", 0),
				act("hello", actions.ActionRemove, "app.network", 0),
				act("hello", actions.ActionRemove, "", 0),
			},
			output: []actions.Action{
				act("hello", actions.ActionStop, "hello.container", 0),
				act("hello", actions.ActionRemove, "hello.container", 0),
				act("hello", actions.ActionRemove, "data.volume", 0),
				act("hello", actions.ActionRemove, "app.network", 0),
				act("hello", actions.ActionRemove, "", 0),
				reload(),
			},
		},
		{
			name: "mixed install update remove with reloads",
			input: []actions.Action{
				act("old", actions.ActionStop, "old.container", 0),
				reload(),
				act("old", actions.ActionRemove, "old.container", 0),
				act("fresh", actions.ActionInstall, "", 0),
				act("fresh", actions.ActionInstall, "hello.container", 0),
				act("existing", actions.ActionUpdate, "world.container", 0),
				reload(),
				act("fresh", actions.ActionStart, "hello.container", 0),
				act("existing", actions.ActionRestart, "world.container", 0),
			},
			output: []actions.Action{
				act("old", actions.ActionStop, "old.container", 0),
				act("existing", actions.ActionUpdate, "world.container", 0),
				act("fresh", actions.ActionInstall, "", 0),
				act("fresh", actions.ActionInstall, "hello.container", 0),
				act("old", actions.ActionRemove, "old.container", 0),
				reload(),
				act("existing", actions.ActionRestart, "world.container", 0),
				act("fresh", actions.ActionStart, "hello.container", 0),
			},
		},
		{
			name: "coalesce restart over start",
			input: []actions.Action{
				act("hello", actions.ActionInstall, "hello.container", 0),
				reload(),
				act("hello", actions.ActionStart, "hello.container", 0),
				act("hello", actions.ActionRestart, "hello.container", 0),
			},
			output: []actions.Action{
				act("hello", actions.ActionInstall, "hello.container", 0),
				reload(),
				act("hello", actions.ActionRestart, "hello.container", 0),
			},
		},
		{
			name: "enable/disable only service action",
			input: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.service", 0),
				act("hello", actions.ActionEnable, "hello.service", 0),
			},
			output: []actions.Action{
				act("hello", actions.ActionInstall, "", 0),
				act("hello", actions.ActionInstall, "hello.service", 0),
				act("hello", actions.ActionEnable, "hello.service", 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearRegistry()
			p := NewPlan()
			assert.Nil(t, p.Append(tt.input))
			result := p.Steps()
			for _, r := range result {
				fmt.Printf("%v %v/%v p: %v\n", r.Todo, r.Parent.Name, r.Target.Path, r.Priority)
			}
			assert.Equal(t, len(tt.output), len(result), "plan is incorrect size: %v != %v", len(tt.output), len(result))
			for k, expectedStep := range tt.output {
				actualStep := result[k]
				assert.Equal(t, expectedStep.Parent.Name, actualStep.Parent.Name, "%v wrong parent %v!=%v", k, expectedStep.Parent, actualStep.Parent)
				assert.Equal(t, expectedStep.Todo, actualStep.Todo, "%v wrong action %v!=%v", k, expectedStep.Todo, actualStep.Todo)
				assert.NotEqual(t, actions.ActionUnknown, actualStep.Todo, "plan contains a unknown action: %+v", actualStep)
				assert.Equal(t, expectedStep.Target, actualStep.Target, "%v wrong target %v!=%v", k, expectedStep.Target, actualStep.Target)
				if expectedStep.Priority != 0 {
					assert.Equal(t, expectedStep.Priority, actualStep.Priority, "%v wrong priority %v != %v", expectedStep.Priority, actualStep.Priority)
				}
			}
		})
	}
}

func mustComponent(name string) *components.Component {
	comp, _ := components.NewComponent(name)
	return comp
}

func account(compname, user string, prio int) actions.Action {
	a := act(compname, actions.ActionDelegate, "", prio)
	a.Target = components.Resource{Parent: compname, Kind: components.ResourceTypeAccount, Path: user}
	return a
}

func Test_PlanDelegateEndOfPlan(t *testing.T) {
	clearRegistry()
	p := NewPlan()
	assert.NoError(t, p.Append([]actions.Action{
		account("root", "webapp", PriorityEndOfPlan),
		act("web", actions.ActionInstall, "web.container", 0),
		account("root", "webapp", PriorityEndOfPlan),
		account("gone", "other", 1),
	}))
	steps := p.Steps()
	assert.Equal(t, 3, p.Size())
	assert.Len(t, steps, 3)
	// the account update with an explicit priority stays with its component
	assert.Equal(t, "other", steps[0].Target.Path)
	assert.Equal(t, 1, steps[0].Priority)
	assert.Equal(t, "web.container", steps[1].Target.Path)
	// the end of plan update comes last, once
	assert.Equal(t, "webapp", steps[2].Target.Path)
	assert.Equal(t, PriorityEndOfPlan, steps[2].Priority)
}

func Test_PlanDelegateNeedsPriority(t *testing.T) {
	clearRegistry()
	p := NewPlan()
	assert.Error(t, p.Add(account("root", "webapp", 0)), "an account update has no default priority")
	assert.Error(t, p.Add(act("web", actions.ActionInstall, "web.container", PriorityEndOfPlan)), "only account updates wait for the end of the plan")
}
