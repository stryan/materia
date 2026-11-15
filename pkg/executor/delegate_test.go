package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/delegate"
	"primamateria.systems/materia/pkg/mocks"
)

func delegatedComponent(name, user string) *components.Component {
	c := mustComponent(name)
	c.State = components.StateFresh
	c.User = user
	c.Delegation = &components.Delegation{
		User:     user,
		Manifest: []byte(""),
		Files:    []components.DelegatedFile{{Path: name + ".container", Content: "[Container]\nImage=x\n"}},
	}
	return c
}

func delegateStep(parent *components.Component, user string) actions.Action {
	return actions.Action{
		Todo:   actions.ActionDelegate,
		Parent: parent,
		Target: components.Resource{Parent: "root", Kind: components.ResourceTypeAccount, Path: user},
	}
}

func delegateTarget(name string) components.Resource {
	return components.Resource{Parent: name, Kind: components.ResourceTypeDelegate, Path: name}
}

func TestDelegate_StageThenUpdate(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "delegate")
	var updated []string
	// no expectations on the host: delegation must not touch this host's Podman, systemd or component repository
	e := NewExecutor(ExecutorConfig{DelegateDir: dir}, mocks.NewMockHostManager(t), 5)
	e.updater = func(_ context.Context, account string) (int64, error) {
		staged := filepath.Join(dir, "accounts", account, "components", "web", "web.container")
		if _, err := os.Stat(staged); err != nil {
			return 0, fmt.Errorf("update ran before the component was staged: %w", err)
		}
		updated = append(updated, account)
		return 3, nil
	}
	comp := delegatedComponent("web", "webapp")
	store := delegate.NewStore(dir)

	require.NoError(t, e.executeAction(ctx, actions.Action{Todo: actions.ActionInstall, Parent: comp, Target: delegateTarget("web")}))
	_, ok, err := store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, e.executeAction(ctx, delegateStep(components.NewRootComponent(), "webapp")))
	assert.Equal(t, []string{"webapp"}, updated)
	_, ok, err = store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.True(t, ok, "a successful update marks the staged components delivered")
}

func TestDelegate_FailedUpdateIsNotDelivered(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "delegate")
	e := NewExecutor(ExecutorConfig{DelegateDir: dir}, mocks.NewMockHostManager(t), 5)
	e.updater = func(context.Context, string) (int64, error) { return 0, errors.New("boom") }
	comp := delegatedComponent("web", "webapp")

	require.NoError(t, e.executeAction(ctx, actions.Action{Todo: actions.ActionInstall, Parent: comp, Target: delegateTarget("web")}))
	err := e.executeAction(ctx, delegateStep(components.NewRootComponent(), "webapp"))
	assert.ErrorContains(t, err, "boom")
	_, ok, derr := delegate.NewStore(dir).Delivered("webapp", "web")
	require.NoError(t, derr)
	assert.False(t, ok, "the next plan must try again")
}

func TestDelegate_RemovalToleratesMissingAccount(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "delegate")
	e := NewExecutor(ExecutorConfig{DelegateDir: dir}, mocks.NewMockHostManager(t), 5)
	e.updater = func(context.Context, string) (int64, error) { return 3, nil }
	comp := delegatedComponent("web", "webapp")
	require.NoError(t, e.executeAction(ctx, actions.Action{Todo: actions.ActionInstall, Parent: comp, Target: delegateTarget("web")}))
	require.NoError(t, e.executeAction(ctx, delegateStep(components.NewRootComponent(), "webapp")))

	// the account was deleted behind materia's back
	e.updater = func(_ context.Context, account string) (int64, error) {
		return 0, fmt.Errorf("%w: %v", delegate.ErrNoAccount, account)
	}
	installed := mustComponent("web")
	installed.User = "webapp"
	installed.State = components.StateNeedRemoval
	require.NoError(t, e.executeAction(ctx, actions.Action{Todo: actions.ActionRemove, Parent: installed, Target: delegateTarget("web")}))
	assert.NoDirExists(t, filepath.Join(dir, "accounts", "webapp", "components", "web"))
	require.NoError(t, e.executeAction(ctx, delegateStep(installed, "webapp")), "removal must not be stuck on an account that is gone")
	_, ok, err := delegate.NewStore(dir).Delivered("webapp", "web")
	require.NoError(t, err)
	assert.False(t, ok)

	// but a missing account is an error when the component should be running
	err = e.executeAction(ctx, delegateStep(components.NewRootComponent(), "webapp"))
	assert.ErrorIs(t, err, delegate.ErrNoAccount)
}

func TestDelegate_NeedsDelegateDir(t *testing.T) {
	e := NewExecutor(ExecutorConfig{}, mocks.NewMockHostManager(t), 5)
	err := e.executeAction(context.Background(), delegateStep(components.NewRootComponent(), "webapp"))
	assert.Error(t, err)
}
