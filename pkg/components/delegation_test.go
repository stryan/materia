package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUser(t *testing.T) {
	for _, name := range []string{"moonshine", "_svc", "forma-build", "a1"} {
		assert.NoError(t, ValidateUser(name), name)
	}
	for _, name := range []string{"", "root", "Moonshine", "1abc", "a/b", "..", "a.b", "a b", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		assert.Error(t, ValidateUser(name), name)
	}
}

func TestDelegation_Digest(t *testing.T) {
	base := func() *Delegation {
		return &Delegation{
			User:       "webapp",
			Manifest:   []byte("m"),
			Files:      []DelegatedFile{{Path: "a.container", Content: "x", Mode: 0o644}},
			Attributes: map[string]any{"b": "1", "a": "2"},
			Repository: DelegatedRepository{Hostname: "forge", Roles: []string{"r"}},
		}
	}
	first, err := base().Digest()
	require.NoError(t, err)
	again, err := base().Digest()
	require.NoError(t, err)
	assert.Equal(t, first, again, "the digest must be stable across runs")

	changes := map[string]func(*Delegation){
		"file content": func(d *Delegation) { d.Files[0].Content = "y" },
		"file mode":    func(d *Delegation) { d.Files[0].Mode = 0o755 },
		"manifest":     func(d *Delegation) { d.Manifest = []byte("n") },
		"attribute":    func(d *Delegation) { d.Attributes["a"] = "3" },
		"roles":        func(d *Delegation) { d.Repository.Roles = nil },
		"hostname":     func(d *Delegation) { d.Repository.Hostname = "other" },
	}
	for name, change := range changes {
		d := base()
		change(d)
		got, err := d.Digest()
		require.NoError(t, err)
		assert.NotEqual(t, first, got, name)
	}

	var none *Delegation
	_, err = none.Digest()
	assert.Error(t, err)
}

func TestComponent_UserValidationAndVersion(t *testing.T) {
	c, err := NewComponent("web")
	require.NoError(t, err)
	c.State = StateFresh
	c.Version = DefaultComponentVersion
	plain, err := c.VersionData()
	require.NoError(t, err)
	assert.NotContains(t, plain.String(), "User", "components without User keep today's version file")

	c.User = "webapp"
	require.NoError(t, c.Validate())
	data, err := c.VersionData()
	require.NoError(t, err)
	assert.Contains(t, data.String(), `User = "webapp"`)
	assert.Contains(t, data.String(), "Version = 1")

	c.User = "root"
	assert.Error(t, c.Validate())
	inst, err := NewComponent("web@one")
	require.NoError(t, err)
	inst.State = StateFresh
	inst.User = "webapp"
	assert.Error(t, inst.Validate())
}
