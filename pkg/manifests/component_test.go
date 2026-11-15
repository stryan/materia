package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeComponentManifests_KeepsSettings(t *testing.T) {
	original, err := LoadComponentManifestFromContent([]byte(`
[Settings]
NoRestart = true
SetupScript = "setup.sh"
CleanupScript = "cleanup.sh"

[Defaults]
port = "8080"
`))
	require.NoError(t, err)

	// an override that says nothing about Settings keeps them
	defaultsOnly, err := LoadComponentManifestFromContent([]byte("[Defaults]\nport = \"80\"\n"))
	require.NoError(t, err)
	merged, err := MergeComponentManifests(original, defaultsOnly)
	require.NoError(t, err)
	assert.Equal(t, "80", merged.Defaults["port"])
	assert.True(t, merged.Settings.GetNoRestart())
	assert.Equal(t, "setup.sh", merged.Settings.GetSetupScript())
	assert.Equal(t, "cleanup.sh", merged.Settings.GetCleanupScript())

	// an override replaces only the settings it sets, and can clear one
	someSettings, err := LoadComponentManifestFromContent([]byte("[Settings]\nNoRestart = false\nCleanupScript = \"\"\n"))
	require.NoError(t, err)
	merged, err = MergeComponentManifests(original, someSettings)
	require.NoError(t, err)
	assert.False(t, merged.Settings.GetNoRestart())
	assert.Equal(t, "setup.sh", merged.Settings.GetSetupScript())
	assert.Empty(t, merged.Settings.GetCleanupScript())
	assert.Equal(t, "8080", merged.Defaults["port"])

	// the original manifest is not modified
	assert.True(t, original.Settings.GetNoRestart())
	assert.Equal(t, "cleanup.sh", original.Settings.GetCleanupScript())
}

func TestSettings_User(t *testing.T) {
	plain, err := LoadComponentManifestFromContent([]byte("[Settings]\nNoRestart = true\n"))
	require.NoError(t, err)
	assert.Equal(t, "", plain.Settings.GetUser())

	m, err := LoadComponentManifestFromContent([]byte("[Settings]\nUser = \"moonshine\"\n"))
	require.NoError(t, err)
	assert.Equal(t, "moonshine", m.Settings.GetUser())

	// an extension can set User for one host
	extended, err := ExtendComponentManifests(plain, m)
	require.NoError(t, err)
	assert.Equal(t, "moonshine", extended.Settings.GetUser())
	assert.True(t, extended.Settings.GetNoRestart())

	// and one that says nothing about User keeps it
	kept, err := ExtendComponentManifests(m, plain)
	require.NoError(t, err)
	assert.Equal(t, "moonshine", kept.Settings.GetUser())
}
