package repository

import (
	"bytes"
	"context"
)

type Repository interface {
	Install(ctx context.Context, path string, data *bytes.Buffer) error
	Remove(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
}
