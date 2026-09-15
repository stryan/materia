package rpc

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/varlink/go/varlink"
	varlinkapi "primamateria.systems/materia/pkg/api"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/source"
)

type VarlinkServer struct {
	materia *materia.Materia
	varlinkapi.VarlinkInterface
}

func (s *VarlinkServer) Facts(ctx context.Context, c varlinkapi.VarlinkCall, hostOnly bool) error {
	log.Info("generating facts on request")
	return c.ReplyFacts(ctx, s.materia.GetFacts(hostOnly))
}

func (s *VarlinkServer) Plan(ctx context.Context, c varlinkapi.VarlinkCall) error {
	log.Info("generating a plan on request")
	plan, err := s.materia.Plan(ctx)
	if err != nil {
		return c.ReplyPlanFailed(ctx, err.Error())
	}
	var result string
	if plan.Empty() {
		planJson, err := plan.ToJson()
		if err != nil {
			return c.ReplyPlanFailed(ctx, err.Error())
		}
		result = string(planJson)
	}
	return c.ReplyPlan(ctx, result, int64(plan.Size()))
}

func (s *VarlinkServer) Sync(ctx context.Context, c varlinkapi.VarlinkCall, revision *string) error {
	log.Info("syncing sources on request")
	opts := &source.SyncOpts{}
	if revision != nil {
		opts.Revision = *revision
	}
	err := s.materia.Source.Sync(ctx, opts)
	if err != nil {
		return c.ReplySyncFailed(ctx, err.Error())
	}
	return c.ReplySync(ctx)
}

func (s *VarlinkServer) Update(ctx context.Context, c varlinkapi.VarlinkCall) error {
	log.Info("running update on request")
	plan, err := s.materia.Plan(ctx)
	if err != nil {
		return c.ReplyPlanFailed(ctx, err.Error())
	}
	rep, err := s.materia.Execute(ctx, plan)
	if err != nil {
		return c.ReplyExecutionFailed(ctx, err.Error(), int64(rep.StepsCompleted), int64(plan.Size()))
	}
	return c.ReplyUpdate(ctx, int64(rep.StepsCompleted))
}

func NewVarlinkServer(ctx context.Context, m *materia.Materia, version string) (*varlink.Service, error) {
	serv, err := varlink.NewService("primamateria", "materia", version, "https://primamateria.systems")
	if err != nil {
		return nil, fmt.Errorf("unable to create varlink service: %w", err)
	}
	if err := serv.RegisterInterface(varlinkapi.VarlinkNew(&VarlinkServer{materia: m})); err != nil {
		return nil, fmt.Errorf("unable to register varlink interface: %w", err)
	}
	return serv, nil
}
