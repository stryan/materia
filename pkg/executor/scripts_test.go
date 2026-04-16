package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/mocks"
	"primamateria.systems/materia/pkg/services"
)

func TestSetupScript(t *testing.T) {
	ctx := context.Background()
	hm := mocks.NewMockHostManager(t)

	comp := &components.Component{
		Name: "test",
	}

	resource := components.Resource{
		Path:   "setup.sh",
		Parent: comp.Name,
		Kind:   components.ResourceTypeScript,
	}

	action := actions.Action{
		Todo:   actions.ActionSetup,
		Target: resource,
		Parent: comp,
	}

	e := &Executor{
		ExecutorConfig: ExecutorConfig{
			ScriptsDir: "/usr/local/bin",
		},
		host:           hm,
		defaultTimeout: 30,
	}

	hm.EXPECT().RunOneshotCommand(ctx, 30, "test-materia-setup.service", []string{"/usr/local/bin/setup.sh"}).Return(nil)
	hm.EXPECT().GetService(ctx, "test-materia-cleanup.service").Return(&services.Service{Name: "test-materia-cleanup.service", State: "active"}, nil)
	hm.EXPECT().ApplyService(ctx, "test-materia-cleanup.service", services.ServiceStop, 30).Return(nil)

	assert.NoError(t, setupScript(ctx, e, action))
}

func TestCleanupScript(t *testing.T) {
	ctx := context.Background()
	hm := mocks.NewMockHostManager(t)

	comp := &components.Component{
		Name: "test",
	}

	resource := components.Resource{
		Path:   "cleanup.sh",
		Parent: comp.Name,
		Kind:   components.ResourceTypeScript,
	}

	action := actions.Action{
		Todo:   actions.ActionCleanup,
		Target: resource,
		Parent: comp,
	}

	e := &Executor{
		ExecutorConfig: ExecutorConfig{
			ScriptsDir: "/usr/local/bin",
		},
		host:           hm,
		defaultTimeout: 30,
	}

	hm.EXPECT().RunOneshotCommand(ctx, 30, "test-materia-cleanup.service", []string{"/usr/local/bin/cleanup.sh"}).Return(nil)
	hm.EXPECT().GetService(mock.Anything, "test-materia-setup.service").Return(&services.Service{Name: "test-materia-setup.service", State: "active"}, nil)
	hm.EXPECT().ApplyService(ctx, "test-materia-setup.service", services.ServiceStop, 30).Return(nil)

	assert.NoError(t, cleanupScript(ctx, e, action))
}
