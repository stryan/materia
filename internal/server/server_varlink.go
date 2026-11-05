package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"charm.land/log/v2"
	"github.com/varlink/go/varlink"
	"primamateria.systems/materia/internal/commands"
	varlinkapi "primamateria.systems/materia/pkg/api"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/source"
)

type VarlinkServer struct {
	srv *Server
	varlinkapi.VarlinkInterface
}

func (s *VarlinkServer) Facts(ctx context.Context, c varlinkapi.VarlinkCall, path *string) error {
	log.Info("generating facts on request")
	var jsonFacts []byte
	err := s.srv.runner(func(m *materia.Materia) error {
		snap, err := facts.Collect(ctx, m.Collectors...)
		if err != nil {
			return fmt.Errorf("unable to get facts: %w", err)
		}

		var out any = snap.Snapshot()
		if path != nil && *path != "" {
			v, err := snap.LookupString(*path)
			if err != nil {
				return err
			}
			// need to make sure it's still an object for the varlink api contract
			out = map[string]any{"path": *path, "value": v}
		}

		jsonFacts, err = json.Marshal(out)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, facts.ErrFactNotFound) {
			return c.ReplyFactNotFound(ctx, *path)
		}
		return c.ReplyFactsFailed(ctx, err.Error())
	}
	return c.ReplyFacts(ctx, jsonFacts)
}

func (s *VarlinkServer) Plan(ctx context.Context, c varlinkapi.VarlinkCall) error {
	log.Info("generating a plan on request")
	var aplan *plan.Plan
	err := s.srv.runner(func(m *materia.Materia) error {
		var err error
		aplan, err = commands.RunPlan(ctx, m, nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return c.ReplyPlanFailed(ctx, err.Error())
	}

	var result string
	if !aplan.Empty() {
		planJson, err := aplan.ToJson()
		if err != nil {
			return c.ReplyPlanFailed(ctx, err.Error())
		}
		result = string(planJson)
	}
	return c.ReplyPlan(ctx, result, int64(aplan.Size()))
}

func (s *VarlinkServer) Sync(ctx context.Context, c varlinkapi.VarlinkCall, revision *string) error {
	log.Info("syncing sources on request")
	opts := &source.SyncOpts{}
	if revision != nil {
		opts.Revision = *revision
	}
	err := s.srv.runner(func(m *materia.Materia) error {
		return m.Source.Sync(ctx, opts)
	})
	if err != nil {
		return c.ReplySyncFailed(ctx, err.Error())
	}

	return c.ReplySync(ctx)
}

func (s *VarlinkServer) Update(ctx context.Context, c varlinkapi.VarlinkCall) error {
	log.Info("running update on request")
	// srv.update already uses runner
	steps, total, err := s.srv.update(ctx, nil)
	if err != nil {
		return c.ReplyUpdateFailed(ctx, err.Error(), steps, total)
	}
	return c.ReplyUpdate(ctx, steps)
}

func NewVarlinkServer(ctx context.Context, srv *Server, version string) (*varlink.Service, error) {
	serv, err := varlink.NewService("primamateria", "materia", version, "https://primamateria.systems")
	if err != nil {
		return nil, fmt.Errorf("unable to create varlink service: %w", err)
	}
	if err := serv.RegisterInterface(varlinkapi.VarlinkNew(&VarlinkServer{srv: srv})); err != nil {
		return nil, fmt.Errorf("unable to register varlink interface: %w", err)
	}
	return serv, nil
}
