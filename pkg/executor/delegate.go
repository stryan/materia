package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/delegate"
)

const (
	// DefaultDelegateTimeout is how long, in seconds, an account's materia may
	// take to update. The delegating materia holds its lock all that time, and
	// the account decides how long its server takes to answer, so keep it short.
	DefaultDelegateTimeout = 300
	// delegateConnectWait is how long to wait for an account's materia to start listening
	delegateConnectWait = 30 * time.Second
)

// accountUpdater asks the materia of an account to update. Tests replace it.
type accountUpdater func(ctx context.Context, account string) (int64, error)

func (e *Executor) delegateStore() (*delegate.Store, error) {
	if e.DelegateDir == "" {
		return nil, errors.New("no delegate directory configured")
	}
	return delegate.NewStore(e.DelegateDir), nil
}

func (e *Executor) updateAccount(ctx context.Context, account string) (int64, error) {
	if e.updater != nil {
		return e.updater(ctx, account)
	}
	timeout := e.DelegateTimeout
	if timeout <= 0 {
		timeout = DefaultDelegateTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	return delegate.UpdateAccount(ctx, account, delegateConnectWait)
}

func stageComponent(ctx context.Context, e *Executor, v actions.Action) error {
	store, err := e.delegateStore()
	if err != nil {
		return err
	}
	return store.Stage(v.Parent)
}

func unstageComponent(ctx context.Context, e *Executor, v actions.Action) error {
	if v.Parent.User == "" {
		return fmt.Errorf("component %v is not delegated", v.Parent.Name)
	}
	store, err := e.delegateStore()
	if err != nil {
		return err
	}
	return store.Unstage(v.Parent.User, v.Parent.Name)
}

func delegateAccount(ctx context.Context, e *Executor, v actions.Action) error {
	account := v.Target.Path
	store, err := e.delegateStore()
	if err != nil {
		return err
	}
	steps, err := e.updateAccount(ctx, account)
	removing := v.Parent != nil && v.Parent.State == components.StateNeedRemoval
	if errors.Is(err, delegate.ErrNoAccount) && removing {
		// nothing runs as an account that is gone, so there is nothing left to remove
		log.Warnf("account %v no longer exists, skipping the update of its materia", account)
		return store.MarkDelivered(account)
	}
	if err != nil {
		return fmt.Errorf("can't delegate to account %v: %w", account, err)
	}
	log.Debugf("materia of account %v completed %v steps", account, steps)
	return store.MarkDelivered(account)
}
