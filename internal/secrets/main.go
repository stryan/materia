package secrets

import (
	"context"

	"github.com/nikolalohinski/gonja/v2/exec"
)

type SecretsManager interface {
	All(context.Context) *exec.Context
}
