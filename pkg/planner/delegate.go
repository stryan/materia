package planner

import (
	"context"
	"errors"
	"fmt"

	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/plan"
)

// DelegationState reports what an account's own materia last applied.
type DelegationState interface {
	// Delivered returns the digest of the component that the account's materia last applied
	Delivered(user, component string) (digest string, ok bool, err error)
}

func isDelegated(tree *ComponentTree) bool {
	return (tree.Source != nil && tree.Source.User != "") || (tree.Host != nil && tree.Host.User != "")
}

// PlanDelegatedComponent plans a component that sets User.
//
// This materia installs only a record of the component: its manifest and its
// lifecycle scripts. Everything else is staged for the account's own materia,
// which installs the Quadlets and manages the services. So no service action
// and no daemon reload is planned here.
//
// Order for a new or changed component: PreScript, the record, the staged
// copy, PostScript, SetupScript, and at the end of the plan one update per
// account. Order for a removed component: remove the staged copy, update the
// account so it stops and removes the component, CleanupScript, then remove
// the record.
func (p *Planner) PlanDelegatedComponent(ctx context.Context, tree *ComponentTree) ([]actions.Action, error) {
	switch {
	case tree.Source == nil:
		return p.planDelegatedRemoval(ctx, tree)
	case tree.Host == nil:
		return p.planDelegatedInstall(tree)
	case tree.Host.User != tree.Source.User:
		return nil, fmt.Errorf("component %v: User changed from %q to %q; remove the component from the host first, then assign it again", tree.Name, tree.Host.User, tree.Source.User)
	default:
		return p.planDelegatedUpdate(ctx, tree)
	}
}

func (p *Planner) planDelegatedInstall(tree *ComponentTree) ([]actions.Action, error) {
	c := tree.Source
	if c.Delegation == nil {
		return nil, fmt.Errorf("component %v sets User but has nothing to delegate", c.Name)
	}
	c.State = components.StateFresh
	steps, err := generateFreshComponentResources(c)
	if err != nil {
		return nil, fmt.Errorf("can't generate fresh resources for %v: %w", tree.Name, err)
	}
	steps = append(steps, stageAction(c))
	tree.FinalState = components.StateFresh
	if p.OnlyResources {
		return steps, nil
	}
	if script := c.Config.GetSetupScript(); script != "" {
		setupResource, err := c.Resources.Get(script)
		if err != nil {
			return nil, fmt.Errorf("setup resource %v not found: %w", script, err)
		}
		steps = append(steps, actions.Action{
			Todo:     actions.ActionSetup,
			Parent:   c,
			Target:   setupResource,
			Priority: 5,
		})
	}
	return append(steps, delegateAction(c.User)), nil
}

func (p *Planner) planDelegatedUpdate(ctx context.Context, tree *ComponentTree) ([]actions.Action, error) {
	host, c := tree.Host, tree.Source
	if c.Delegation == nil {
		return nil, fmt.Errorf("component %v sets User but has nothing to delegate", c.Name)
	}
	host.State = components.StateMayNeedUpdate
	c.State = components.StateFresh
	dropSecrets(host)

	resourceActions, err := generateUpdatedComponentResources(ctx, p.Host, p.PlannerConfig, host, c)
	if err != nil {
		return nil, fmt.Errorf("can't generate resources for %v: %w", tree.Name, err)
	}
	if host.Version != components.DefaultComponentVersion {
		host.Version = components.DefaultComponentVersion
		resourceActions = append(resourceActions, actions.Action{
			Todo:   actions.ActionUpdate,
			Parent: host,
			Target: components.Resource{Parent: c.Name, Kind: components.ResourceTypeComponent, Path: c.Name},
		})
	}

	digest, err := c.Delegation.Digest()
	if err != nil {
		return nil, err
	}
	delivered := false
	if p.Delegation != nil {
		last, ok, err := p.Delegation.Delivered(c.User, c.Name)
		if err != nil {
			return nil, fmt.Errorf("can't read delegation state of %v: %w", tree.Name, err)
		}
		delivered = ok && last == digest
	}
	if len(resourceActions) == 0 && delivered {
		host.State = components.StateOK
		tree.FinalState = components.StateOK
		return nil, nil
	}
	host.State = components.StateNeedUpdate
	tree.FinalState = components.StateNeedUpdate

	var steps []actions.Action
	if script := c.Config.GetPreScript(); script != "" && !p.OnlyResources {
		preResource, err := c.Resources.Get(script)
		if err != nil {
			return nil, fmt.Errorf("pre-update resource %v not found: %w", script, err)
		}
		steps = append(steps, actions.Action{
			Todo:     actions.ActionExecute,
			Parent:   c,
			Target:   preResource,
			Priority: 1,
		}.With(actions.WithOneshotName(fmt.Sprintf("%v-materia-pre-update.service", c.Name))))
	}
	steps = append(steps, resourceActions...)
	steps = append(steps, stageAction(c))
	if p.OnlyResources {
		return steps, nil
	}
	if script := c.Config.GetPostScript(); script != "" {
		postResource, err := c.Resources.Get(script)
		if err != nil {
			return nil, fmt.Errorf("post-update resource %v not found: %w", script, err)
		}
		steps = append(steps, actions.Action{
			Todo:     actions.ActionExecute,
			Parent:   c,
			Target:   postResource,
			Priority: 4,
		}.With(actions.WithOneshotName(fmt.Sprintf("%v-materia-post-update.service", c.Name))))
	}
	return append(steps, delegateAction(c.User)), nil
}

func (p *Planner) planDelegatedRemoval(ctx context.Context, tree *ComponentTree) ([]actions.Action, error) {
	host := tree.Host
	if host.User == "" {
		return nil, errors.New("expected a delegated component")
	}
	host.State = components.StateNeedRemoval
	dropSecrets(host)

	steps := []actions.Action{{
		Todo:   actions.ActionRemove,
		Parent: host,
		Target: components.Resource{Parent: host.Name, Kind: components.ResourceTypeDelegate, Path: host.Name},
	}}
	if !p.OnlyResources {
		// The account's materia must remove the component before the cleanup
		// script runs, because that script may delete the account. So this
		// update can't wait for the end of the plan.
		now := delegateAction(host.User)
		now.Parent = host
		now.Priority = 1
		steps = append(steps, now)
		if script := host.Config.GetCleanupScript(); script != "" {
			cleanupResource, err := host.Resources.Get(script)
			if err != nil {
				return nil, fmt.Errorf("cleanup resource %v not found: %w", script, err)
			}
			steps = append(steps, actions.Action{
				Todo:     actions.ActionCleanup,
				Parent:   host,
				Target:   cleanupResource,
				Priority: 2,
			})
		}
	}
	resourceActions, err := generateRemovedComponentResources(ctx, p.Host, p.PlannerConfig, host)
	if err != nil {
		return nil, fmt.Errorf("can't generate removed resources for %v: %w", tree.Name, err)
	}
	tree.FinalState = components.StateNeedRemoval
	return append(steps, resourceActions...), nil
}

func stageAction(c *components.Component) actions.Action {
	return actions.Action{
		Todo:   actions.ActionInstall,
		Parent: c,
		Target: components.Resource{Parent: c.Name, Kind: components.ResourceTypeDelegate, Path: c.Name},
	}
}

// delegateAction updates user's materia once the rest of the plan has run.
func delegateAction(user string) actions.Action {
	return actions.Action{
		Todo:     actions.ActionDelegate,
		Parent:   components.NewRootComponent(),
		Target:   components.Resource{Parent: "root", Kind: components.ResourceTypeAccount, Path: user},
		Priority: plan.PriorityEndOfPlan,
	}
}

// dropSecrets removes Podman secrets from an installed delegated component.
// Its manifest may list secrets, but they belong to the account's Podman.
// Without this, a secret of the same name in this materia's Podman would be
// planned for removal.
func dropSecrets(c *components.Component) {
	for _, r := range c.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			c.Resources.Delete(r.Path)
		}
	}
}
