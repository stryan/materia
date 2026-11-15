package planner

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
)

type fakeDelivery map[string]string

func (f fakeDelivery) Delivered(user, component string) (string, bool, error) {
	digest, ok := f[user+"/"+component]
	return digest, ok, nil
}

func scriptRes(parent, path string) components.Resource {
	return components.Resource{Parent: parent, Path: path, Kind: components.ResourceTypeScript, Content: "#!/bin/sh\ntrue\n", Mode: 0o755}
}

func manifestRes(parent, content string) components.Resource {
	return components.Resource{Parent: parent, Path: manifests.ComponentManifestFile, Kind: components.ResourceTypeManifest, Content: content, Mode: 0o644}
}

var allScripts = manifests.Settings{
	User:          new("webapp"),
	SetupScript:   new("setup.sh"),
	CleanupScript: new("cleanup.sh"),
	PreScript:     new("pre.sh"),
	PostScript:    new("post.sh"),
}

// delegatedSource is what the loader produces for a component with User.
func delegatedSource(name, user, content string) *components.Component {
	settings := allScripts
	settings.User = new(user)
	return &components.Component{
		Name:  name,
		State: components.StateFresh,
		User:  user,
		Resources: newResSet(manifestRes(name, "manifest"), scriptRes(name, "setup.sh"), scriptRes(name, "cleanup.sh"),
			scriptRes(name, "pre.sh"), scriptRes(name, "post.sh")),
		ServiceManifests: newServManSet(),
		Config:           settings,
		Version:          components.DefaultComponentVersion,
		Delegation: &components.Delegation{
			User:     user,
			Manifest: []byte("manifest"),
			Files:    []components.DelegatedFile{{Path: name + ".container", Content: content}},
		},
	}
}

// delegatedHost is the record of an installed delegated component.
func delegatedHost(name, user string) *components.Component {
	c := delegatedSource(name, user, "")
	c.State = components.StateStale
	c.Delegation = nil
	// the installed manifest lists a secret; it belongs to the account's Podman
	_ = c.Resources.Add(components.Resource{Parent: name, Path: "token", Kind: components.ResourceTypePodmanSecret, Content: "x"})
	return c
}

func pretty(steps []actions.Action) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Pretty())
	}
	return out
}

func mustDigest(t *testing.T, c *components.Component) string {
	t.Helper()
	digest, err := c.Delegation.Digest()
	require.NoError(t, err)
	return digest
}

func TestPlanDelegated_Fresh(t *testing.T) {
	// no expectations: a delegated component must not query this host's Podman or systemd
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{}

	plan, err := p.Plan(context.Background(), nil, []*components.Component{delegatedSource("web", "webapp", "v1")})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"(web) Install Component web",
		"(web) Install Manifest MANIFEST.toml",
		"(web) Install Script setup.sh",
		"(web) Install Script cleanup.sh",
		"(web) Install Script pre.sh",
		"(web) Install Script post.sh",
		"(web) Install Delegate web",
		"(web) Setup Script setup.sh",
		"(root) Delegate Account webapp",
	}, pretty(plan.Steps()))
	assert.Equal(t, 9, plan.Size())
}

func TestPlanDelegated_UnchangedPlansNothing(t *testing.T) {
	src := delegatedSource("web", "webapp", "v1")
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{"webapp/web": mustDigest(t, src)}

	plan, err := p.Plan(context.Background(), []*components.Component{delegatedHost("web", "webapp")}, []*components.Component{src})
	require.NoError(t, err)
	assert.True(t, plan.Empty(), "got %v", pretty(plan.Steps()))
}

func TestPlanDelegated_ChangedFileRunsScriptsAndDelegates(t *testing.T) {
	old := delegatedSource("web", "webapp", "v1")
	src := delegatedSource("web", "webapp", "v2")
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{"webapp/web": mustDigest(t, old)}

	plan, err := p.Plan(context.Background(), []*components.Component{delegatedHost("web", "webapp")}, []*components.Component{src})
	require.NoError(t, err)
	// only a staged file changed: the record is untouched, but the scripts still run
	assert.Equal(t, []string{
		"(web) Execute Script pre.sh",
		"(web) Install Delegate web",
		"(web) Execute Script post.sh",
		"(root) Delegate Account webapp",
	}, pretty(plan.Steps()))
}

func TestPlanDelegated_RetriesUndelivered(t *testing.T) {
	// staged earlier, but the account's materia never confirmed: plan it again
	src := delegatedSource("web", "webapp", "v1")
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{}

	plan, err := p.Plan(context.Background(), []*components.Component{delegatedHost("web", "webapp")}, []*components.Component{src})
	require.NoError(t, err)
	steps := pretty(plan.Steps())
	assert.Contains(t, steps, "(web) Install Delegate web")
	assert.Equal(t, "(root) Delegate Account webapp", steps[len(steps)-1])
}

func TestPlanDelegated_Removal(t *testing.T) {
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{}

	plan, err := p.Plan(context.Background(), []*components.Component{delegatedHost("web", "webapp")}, nil)
	require.NoError(t, err)
	steps := pretty(plan.Steps())
	// the account removes the component before the cleanup script may delete the account
	assert.Equal(t, []string{
		"(web) Remove Delegate web",
		"(web) Delegate Account webapp",
		"(web) Cleanup Script cleanup.sh",
	}, steps[:3])
	assert.Equal(t, "(web) Remove Component web", steps[len(steps)-1])
	assert.NotContains(t, steps, "(web) Remove PodmanSecret token", "a secret of the account must never be removed from this host's Podman")
	for _, s := range steps {
		assert.NotContains(t, s, "(root)", "no update is left for the end of the plan")
	}
}

func TestPlanDelegated_SharedAccount(t *testing.T) {
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{}

	plan, err := p.Plan(context.Background(), nil, []*components.Component{
		delegatedSource("moonshine", "moonshine", "a"),
		delegatedSource("nvidia-driver", "moonshine", "b"),
		delegatedSource("other", "someone", "c"),
	})
	require.NoError(t, err)
	steps := pretty(plan.Steps())
	// both components are staged and set up before the one update of their account
	assert.Equal(t, []string{"(root) Delegate Account moonshine", "(root) Delegate Account someone"}, steps[len(steps)-2:])
	count := 0
	for _, s := range steps {
		if s == "(root) Delegate Account moonshine" {
			count++
		}
	}
	assert.Equal(t, 1, count)
	assert.Equal(t, len(steps), plan.Size())
}

func TestPlanDelegated_RemoveAndInstallOnOneAccount(t *testing.T) {
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	p.Delegation = fakeDelivery{}

	plan, err := p.Plan(context.Background(),
		[]*components.Component{delegatedHost("gone", "webapp")},
		[]*components.Component{delegatedSource("web", "webapp", "v1")})
	require.NoError(t, err)
	steps := pretty(plan.Steps())
	index := func(want string) int {
		for i, s := range steps {
			if s == want {
				return i
			}
		}
		t.Fatalf("%q not in plan %v", want, steps)
		return -1
	}
	assert.Less(t, index("(gone) Delegate Account webapp"), index("(gone) Cleanup Script cleanup.sh"))
	assert.Less(t, index("(web) Setup Script setup.sh"), index("(root) Delegate Account webapp"))
}

func TestPlanDelegated_OnlyResources(t *testing.T) {
	p := NewPlanner(PlannerConfig{OnlyResources: true}, mocks.NewMockHostManager(t))
	plan, err := p.Plan(context.Background(), nil, []*components.Component{delegatedSource("web", "webapp", "v1")})
	require.NoError(t, err)
	for _, s := range pretty(plan.Steps()) {
		assert.NotContains(t, s, "Setup")
		assert.NotContains(t, s, "Delegate Account")
	}
}

func TestPlanDelegated_UserChangeRejected(t *testing.T) {
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	cases := map[string]*components.Component{
		"other account": delegatedHost("web", "other"),
		"was installed by this materia": func() *components.Component {
			c := delegatedHost("web", "")
			c.User = ""
			return c
		}(),
	}
	for name, host := range cases {
		_, err := p.Plan(context.Background(), []*components.Component{host}, []*components.Component{delegatedSource("web", "webapp", "v1")})
		assert.ErrorContains(t, err, "User changed", name)
	}
	// and the other way: a delegated component that drops User
	plain := delegatedSource("web", "webapp", "v1")
	plain.User, plain.Delegation = "", nil
	_, err := p.Plan(context.Background(), []*components.Component{delegatedHost("web", "webapp")}, []*components.Component{plain})
	assert.ErrorContains(t, err, "User changed")
}

func TestPlanDelegated_PassesPlanValidation(t *testing.T) {
	// guards against the validators rejecting the new action kinds
	p := NewPlanner(PlannerConfig{}, mocks.NewMockHostManager(t))
	plan, err := p.Plan(context.Background(), nil, []*components.Component{delegatedSource("web", "webapp", "v1")})
	require.NoError(t, err)
	for i, a := range plan.Steps() {
		require.NoError(t, a.Validate(), fmt.Sprintf("step %v", i))
	}
}
