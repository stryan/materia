package materia

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/manifests"
)

func TestNew(t *testing.T) {
	hm := NewMockHostManager(t)
	sm := NewMockSourceManager(t)
	sm.EXPECT().LoadManifest(manifests.MateriaManifestFile).Return(&manifests.MateriaManifest{}, nil)
	hm.EXPECT().GetHostname().Return("localhost")
	m, err := NewMateriaFromConfig(context.TODO(), &MateriaConfig{
		QuadletDir: "/tmp/materia/quadlets",
		MateriaDir: "/tmp/materia",
		ServiceDir: "/tmp/services",
		ScriptsDir: "/usr/local/bin",
		SourceDir:  "/materia/source",
	}, hm, sm)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}
