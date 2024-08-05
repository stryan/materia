package source

import "context"

type Source interface {
	Sync(context.Context) error
}
