package loader

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

// DelegationStage splits a component that sets User into two parts.
//
// The first part goes to the account's own materia: every file of the
// component as it is in the source, the merged manifest and the attributes.
// The stage stores it in comp.Delegation.
//
// The second part stays in comp.Resources and is what this materia installs
// itself: the manifest and the lifecycle scripts, which run as this materia's
// user. It is the record that the component is installed.
//
// The stage does nothing for a component without User.
type DelegationStage struct {
	manager    components.ComponentReader
	attrs      map[string]any
	overrides  []*manifests.ComponentManifest
	extensions []*manifests.ComponentManifest
}

func (s *DelegationStage) Process(ctx context.Context, comp *components.Component) error {
	user := comp.Config.GetUser()
	if user == "" {
		return nil
	}
	if err := components.ValidateUser(user); err != nil {
		return fmt.Errorf("component %v: %w", comp.Name, err)
	}
	if comp.Instance != "" {
		return fmt.Errorf("component %v: instanced components can't set User", comp.InstanceName())
	}

	manifest, err := mergedManifest(s.manager, comp, s.overrides, s.extensions)
	if err != nil {
		return err
	}
	lifecycle := lifecycleScripts(manifest.Settings)
	staged, err := stagedManifest(manifest)
	if err != nil {
		return fmt.Errorf("can't encode manifest for %v's account: %w", comp.Name, err)
	}

	delegation := &components.Delegation{
		User:       user,
		Manifest:   staged,
		Attributes: maps.Clone(s.attrs),
	}
	for _, r := range comp.Resources.List() {
		isLifecycle := slices.Contains(lifecycle, r.Path)
		switch {
		case r.Kind == components.ResourceTypeManifest && r.Path == manifests.ComponentManifestFile:
			// kept as the record; the account gets the merged manifest instead
			continue
		case r.Kind == components.ResourceTypePodmanSecret:
			// the account's materia creates the secret from the staged attributes
			comp.Resources.Delete(r.Path)
			continue
		case isLifecycle:
			// runs as this materia's user, never in the account
			continue
		case isDir(r) && holdsAny(r.Path, lifecycle):
			// parent directory of a lifecycle script: needed on both sides
		default:
			comp.Resources.Delete(r.Path)
		}
		delegation.Files = append(delegation.Files, components.DelegatedFile{
			Path:    r.Filepath(),
			Mode:    r.Mode.Perm(),
			Dir:     isDir(r),
			Content: r.Content,
		})
	}

	comp.User = user
	comp.Delegation = delegation
	// the account's materia manages the services
	comp.ServiceManifests = components.NewServiceManifestSet()
	return nil
}

func isDir(r components.Resource) bool {
	return r.Kind == components.ResourceTypeDirectory || r.Kind == components.ResourceTypeDropinDir
}

func holdsAny(dir string, paths []string) bool {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	return slices.ContainsFunc(paths, func(p string) bool {
		return strings.HasPrefix(p, prefix)
	})
}

func lifecycleScripts(s manifests.Settings) []string {
	var scripts []string
	for _, script := range []string{s.GetSetupScript(), s.GetCleanupScript(), s.GetPreScript(), s.GetPostScript()} {
		if script != "" && !slices.Contains(scripts, script) {
			scripts = append(scripts, script)
		}
	}
	return scripts
}

// stagedManifest returns the manifest the account's materia reads. It drops
// User, so that materia does not delegate again, and the lifecycle scripts,
// which only the delegating materia runs.
func stagedManifest(manifest *manifests.ComponentManifest) ([]byte, error) {
	staged := *manifest
	staged.Settings.User = nil
	staged.Settings.SetupScript = nil
	staged.Settings.CleanupScript = nil
	staged.Settings.PreScript = nil
	staged.Settings.PostScript = nil
	lifecycle := lifecycleScripts(manifest.Settings)
	staged.Scripts = slices.DeleteFunc(slices.Clone(manifest.Scripts), func(script string) bool {
		return slices.Contains(lifecycle, script)
	})
	buffer := bytes.NewBuffer(nil)
	if err := toml.NewEncoder(buffer).Encode(staged); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
