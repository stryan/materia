package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type remote struct {
	path, subpath string
}

type RemoteComponentRegistry struct {
	root    string
	remotes map[string]remote
	lock    sync.RWMutex
}

func NewRemoteComponentRegistry(remoteDir string) *RemoteComponentRegistry {
	return &RemoteComponentRegistry{
		root:    remoteDir,
		remotes: map[string]remote{},
	}
}

func (r *RemoteComponentRegistry) ClonePath(name string) string {
	return filepath.Join(r.root, "components", name)
}

// Needs a synced remote
func (r *RemoteComponentRegistry) Register(name, subpath string) error {
	if err := components.ValidateComponentName(name); err != nil {
		return err
	}
	if _, err := resolveRemote(r.ClonePath(name), subpath); err != nil {
		return fmt.Errorf("invalid remote component %v: %w", name, err)
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.remotes[name] = remote{r.ClonePath(name), subpath}
	return nil
}

func (r *RemoteComponentRegistry) Resolve(name string) (dir string, found bool, err error) {
	r.lock.RLock()
	e, ok := r.remotes[name]
	r.lock.RUnlock()
	if !ok {
		return "", false, nil
	}
	dir, err = resolveRemote(e.path, e.subpath)
	return dir, true, err
}

func (r *RemoteComponentRegistry) Size() int {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return len(r.remotes)
}

func (r *RemoteComponentRegistry) List() []string {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return slices.Collect(maps.Keys(r.remotes))
}

func resolveRemote(clone, subpath string) (string, error) {
	if subpath != "" && !filepath.IsLocal(subpath) {
		return "", fmt.Errorf("invalid subpath %q", subpath)
	}
	dir := clone
	for part := range strings.SplitSeq(filepath.ToSlash(subpath), "/") {
		if part == "" || part == "." {
			continue
		}
		dir = filepath.Join(dir, part)
		info, err := lstatSource(dir)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("subpath element %v is not a directory", dir)
		}
	}
	info, err := lstatSource(filepath.Join(dir, manifests.ComponentManifestFile))
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%v is not a regular file", manifests.ComponentManifestFile)
	}
	return dir, nil
}

func (r *RemoteComponentRegistry) Prune() error {
	if r.root == "" {
		return nil
	}
	root, err := os.OpenRoot(filepath.Join(r.root, "components"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	r.lock.RLock()
	defer r.lock.RUnlock()
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := r.remotes[e.Name()]; ok {
			continue
		}
		log.Debugf("Removing old remote component %v", e.Name())
		if err := root.RemoveAll(e.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *RemoteComponentRegistry) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	clear(r.remotes)
}

func (r *RemoteComponentRegistry) Clean() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.root == "" {
		return nil
	}
	root, err := os.OpenRoot(filepath.Join(r.root, "components"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.RemoveAll("components")
}
