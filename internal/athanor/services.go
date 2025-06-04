package athanor

import (
	"context"

	"git.saintnet.tech/stryan/materia/internal/services"
)

type Services interface {
	Apply(context.Context, string, services.ServiceAction) error
	Get(context.Context, string) (*services.Service, error)
	WaitUntilState(context.Context, string, string) error
	Close()
}
