package materia

import (
	"context"
)

type Source interface {
	Sync(context.Context) error
	Close(context.Context) error
	Clean() error
}
