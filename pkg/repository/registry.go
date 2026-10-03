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

var ErrNoRemoteDir = errors.New("no remote dir configured")

type remote struct {
	path, subpath string
}

type RemoteComponentRegistry struct {
	root    string
	remotes map[string]remote
	lock    sync.RWMutex
}

func NewRemoteComponentRegistry(remoteDir string) (*RemoteComponentRegistry, error) {
	if remoteDir != "" {
		abs, err := filepath.Abs(remoteDir)
		if err != nil {
			return nil, err
		}
		remoteDir = abs
	}
	return &RemoteComponentRegistry{
		root:    remoteDir,
		remotes: map[string]remote{},
	}, nil
}

func (r *RemoteComponentRegistry) ClonePath(name string) (string, error) {
	if err := components.ValidateComponentName(name); err != nil {
		return "", err
	}
	dir, err := r.componentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Needs a synced remote
func (r *RemoteComponentRegistry) Register(name, subpath string) error {
	path, err := r.ClonePath(name)
	if err != nil {
		return err
	}
	if _, err := resolveRemote(path, subpath); err != nil {
		return fmt.Errorf("invalid remote component %v: %w", name, err)
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.remotes[name] = remote{path, subpath}
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
	info, err := lstatSource(clone)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("remote clone %v is not a directory", clone)
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
	info, err = lstatSource(filepath.Join(dir, manifests.ComponentManifestFile))
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%v is not a regular file", manifests.ComponentManifestFile)
	}
	return dir, nil
}

func (r *RemoteComponentRegistry) Prune() error {
	dir, err := r.componentsDir()
	if errors.Is(err, ErrNoRemoteDir) {
		return nil
	}
	if err != nil {
		return err
	}

	root, err := os.OpenRoot(dir)
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
	dir, err := r.componentsDir()
	if errors.Is(err, ErrNoRemoteDir) {
		return nil
	}
	if err != nil {
		return err
	}

	clear(r.remotes)
	return os.RemoveAll(dir)
}

func (r *RemoteComponentRegistry) componentsDir() (string, error) {
	if r.root == "" {
		return "", ErrNoRemoteDir
	}
	return filepath.Join(r.root, "components"), nil
}
