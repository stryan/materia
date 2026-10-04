package hostman

import (
	"context"
)

type Repository interface {
	Install(ctx context.Context, path string, data []byte) error
	Remove(ctx context.Context, path string) error
}
