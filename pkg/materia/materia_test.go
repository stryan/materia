package materia

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/pkg/attributes/mem"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
)

func TestNew(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	sm := mocks.NewMockSourceManager(t)
	sm.EXPECT().LoadManifest(manifests.MateriaManifestFile).Return(&manifests.MateriaManifest{}, nil)
	hm.EXPECT().GetHostname().Return("localhost")
	engine := mem.NewMemoryEngine()
	m, err := New(context.Background(), &MateriaConfig{
		QuadletDir: "/tmp/materia/quadlets",
		MateriaDir: "/tmp/materia",
		ServiceDir: "/tmp/services",
		ScriptsDir: "/usr/local/bin",
		SourceDir:  "/materia/source",
	}, hm, sm, engine)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}
