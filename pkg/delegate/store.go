// Package delegate hands components to an account's own materia.
//
// A component that sets User in its manifest is not installed by the materia
// that reads the repository. That materia writes the component into a staging
// directory, shaped like a materia repository, and then asks the account's
// materia to update from it.
//
// Layout below the delegate directory:
//
//	accounts/<user>/MANIFEST.toml               generated; assigns every staged component
//	accounts/<user>/components/<name>/          the component, templates not rendered
//	accounts/<user>/attributes/<name>.toml      the attributes resolved for the component
//	state/<user>.toml                           what is staged and what the account applied
package delegate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/sys/unix"
	"primamateria.systems/materia/pkg/attributes"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

const (
	accountsDir   = "accounts"
	stateDir      = "state"
	componentsDir = "components"
	// AttributesDir is the file attributes base_dir the account's materia must use.
	AttributesDir = "attributes"
)

// Store is the staging area for delegated components.
type Store struct {
	dir string
}

// NewStore returns the store rooted at dir. It creates nothing until a
// component is staged.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// AccountDir is the repository staged for user. The account's materia uses it
// as a local source.
func (s *Store) AccountDir(user string) string {
	return filepath.Join(s.dir, accountsDir, user)
}

type state struct {
	// Staged maps a component to the digest of its staged copy
	Staged map[string]string
	// Delivered maps a component to the digest the account's materia last applied
	Delivered map[string]string
}

type repoManifest struct {
	Hosts    map[string]repoHost       `toml:"Hosts"`
	Snippets []manifests.SnippetConfig `toml:"Snippets,omitempty"`
}

type repoHost struct {
	Components []string `toml:"Components,omitempty"`
	Roles      []string `toml:"Roles,omitempty"`
}

func (s *Store) statePath(user string) string {
	return filepath.Join(s.dir, stateDir, user+".toml")
}

func (s *Store) loadState(user string) (state, error) {
	st := state{Staged: map[string]string{}, Delivered: map[string]string{}}
	data, err := os.ReadFile(s.statePath(user))
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := toml.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("invalid delegation state for %v: %w", user, err)
	}
	if st.Staged == nil {
		st.Staged = map[string]string{}
	}
	if st.Delivered == nil {
		st.Delivered = map[string]string{}
	}
	return st, nil
}

func (s *Store) saveState(user string, st state) error {
	if err := os.MkdirAll(filepath.Join(s.dir, stateDir), 0o700); err != nil {
		return err
	}
	return writeTOML(s.statePath(user), 0o600, st)
}

// Delivered returns the digest of component that user's materia last applied.
func (s *Store) Delivered(user, component string) (string, bool, error) {
	if err := components.ValidateUser(user); err != nil {
		return "", false, err
	}
	st, err := s.loadState(user)
	if err != nil {
		return "", false, err
	}
	digest, ok := st.Delivered[component]
	return digest, ok, nil
}

// MarkDelivered records that user's materia applied everything now staged.
func (s *Store) MarkDelivered(user string) error {
	if err := components.ValidateUser(user); err != nil {
		return err
	}
	st, err := s.loadState(user)
	if err != nil {
		return err
	}
	if maps.Equal(st.Staged, st.Delivered) {
		return nil
	}
	st.Delivered = maps.Clone(st.Staged)
	return s.saveState(user, st)
}

// Stage writes comp into its account's repository. Staging the same
// component again replaces the earlier copy in one step, so the account's
// materia never reads half a component.
func (s *Store) Stage(comp *components.Component) error {
	d := comp.Delegation
	if d == nil {
		return fmt.Errorf("component %v has nothing to delegate", comp.Name)
	}
	if err := components.ValidateUser(d.User); err != nil {
		return err
	}
	if err := components.ValidateComponentName(comp.Name); err != nil {
		return err
	}
	digest, err := d.Digest()
	if err != nil {
		return err
	}
	account, err := s.ensureAccountDir(d.User)
	if err != nil {
		return err
	}
	if err := writeComponent(filepath.Join(account, componentsDir), comp.Name, d); err != nil {
		return fmt.Errorf("can't stage component %v for %v: %w", comp.Name, d.User, err)
	}
	if err := writeAttributes(filepath.Join(account, AttributesDir), comp.Name, d.Attributes); err != nil {
		return fmt.Errorf("can't stage attributes of %v for %v: %w", comp.Name, d.User, err)
	}
	if err := s.writeRepoManifest(account, &d.Repository); err != nil {
		return err
	}
	st, err := s.loadState(d.User)
	if err != nil {
		return err
	}
	st.Staged[comp.Name] = digest
	return s.saveState(d.User, st)
}

// Unstage removes component from user's repository. It is not an error if
// the component was never staged.
func (s *Store) Unstage(user, component string) error {
	if err := components.ValidateUser(user); err != nil {
		return err
	}
	if err := components.ValidateComponentName(component); err != nil {
		return err
	}
	account := s.AccountDir(user)
	if _, err := os.Stat(account); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	compDir := filepath.Join(account, componentsDir, component)
	// move it out of sight first, so a sync never copies half of it
	doomed := filepath.Join(account, componentsDir, "."+component+".old")
	if err := os.RemoveAll(doomed); err != nil {
		return err
	}
	if err := os.Rename(compDir, doomed); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.RemoveAll(doomed); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(account, AttributesDir, component+".toml")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := s.writeRepoManifest(account, nil); err != nil {
		return err
	}
	st, err := s.loadState(user)
	if err != nil {
		return err
	}
	delete(st.Staged, component)
	return s.saveState(user, st)
}

// ensureAccountDir creates the account's repository. The top directory is
// created with mode 0750 and is never changed afterwards: whoever sets up the
// account gives it read access, for example by changing the group.
func (s *Store) ensureAccountDir(user string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(s.dir), 0o755); err != nil {
		return "", err
	}
	for _, dir := range []string{s.dir, filepath.Join(s.dir, accountsDir)} {
		if err := mkdirMode(dir, 0o755); err != nil {
			return "", err
		}
	}
	account := s.AccountDir(user)
	if err := mkdirMode(account, 0o750); err != nil {
		return "", err
	}
	for _, sub := range []string{componentsDir, AttributesDir} {
		if err := mkdirMode(filepath.Join(account, sub), 0o755); err != nil {
			return "", err
		}
	}
	return account, nil
}

// mkdirMode creates dir with exactly mode, whatever the umask is. It leaves
// a directory that already exists as it is.
func mkdirMode(dir string, mode fs.FileMode) error {
	if err := os.Mkdir(dir, mode); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil
		}
		return err
	}
	return os.Chmod(dir, mode)
}

// openModes gives every directory below dir mode 0755 and every file 0644,
// or 0755 if its owner may execute it. The account reads the staged files as
// "other", so their modes must not depend on the umask.
func openModes(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if d.IsDir() {
			mode = 0o755
		} else {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Mode()&0o100 != 0 {
				mode = 0o755
			}
		}
		return os.Chmod(path, mode)
	})
}

func writeComponent(parent, name string, d *components.Delegation) error {
	fresh := filepath.Join(parent, "."+name+".new")
	if err := os.RemoveAll(fresh); err != nil {
		return err
	}
	if err := os.Mkdir(fresh, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(fresh)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.WriteFile(manifests.ComponentManifestFile, d.Manifest, 0o644); err != nil {
		return err
	}
	for _, f := range d.Files {
		if !filepath.IsLocal(f.Path) {
			return fmt.Errorf("invalid path in component: %v", f.Path)
		}
		if f.Dir {
			if err := root.MkdirAll(f.Path, 0o755); err != nil {
				return err
			}
			continue
		}
		if dir := filepath.Dir(f.Path); dir != "." {
			if err := root.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		mode := fs.FileMode(0o644)
		if f.Mode&0o111 != 0 {
			mode = 0o755
		}
		if err := root.WriteFile(f.Path, []byte(f.Content), mode); err != nil {
			return err
		}
	}
	if err := openModes(fresh); err != nil {
		return err
	}
	current := filepath.Join(parent, name)
	err = unix.Renameat2(unix.AT_FDCWD, fresh, unix.AT_FDCWD, current, unix.RENAME_EXCHANGE)
	switch {
	case err == nil:
		// fresh now holds the previous copy
		return os.RemoveAll(fresh)
	case errors.Is(err, fs.ErrNotExist):
		return os.Rename(fresh, current)
	default:
		return err
	}
}

func writeAttributes(dir, component string, attrs map[string]any) error {
	path := filepath.Join(dir, component+".toml")
	if len(attrs) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	vault := attributes.AttributeVault{
		Components: map[string]map[string]any{component: attrs},
	}
	// The account reads this file as "other": only the top directory has its
	// group. The mode of that directory is what keeps everyone else out.
	return writeTOML(path, 0o644, vault)
}

// writeRepoManifest regenerates the account's repository manifest from the
// components now staged. With a nil repo it keeps the host name, roles and
// snippets of the manifest that is already there.
func (s *Store) writeRepoManifest(account string, repo *components.DelegatedRepository) error {
	path := filepath.Join(account, manifests.MateriaManifestFile)
	staged, err := stagedComponents(account)
	if err != nil {
		return err
	}
	manifest := repoManifest{Hosts: map[string]repoHost{}}
	if repo != nil {
		manifest.Snippets = repo.Snippets
		if repo.Hostname != "" && len(repo.Roles) > 0 {
			manifest.Hosts[repo.Hostname] = repoHost{Roles: repo.Roles}
		}
	} else {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err == nil {
			if err := toml.Unmarshal(data, &manifest); err != nil {
				return fmt.Errorf("invalid staged repository manifest %v: %w", path, err)
			}
			if manifest.Hosts == nil {
				manifest.Hosts = map[string]repoHost{}
			}
		}
	}
	all := manifest.Hosts["all"]
	all.Components = staged
	manifest.Hosts["all"] = all
	return writeTOML(path, 0o644, manifest)
}

func stagedComponents(account string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(account, componentsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// writeTOML writes value to path through a temporary file and a rename, so a
// reader sees either the old file or the new one.
func writeTOML(path string, mode fs.FileMode, value any) error {
	buffer := bytes.NewBuffer(nil)
	if err := toml.NewEncoder(buffer).Encode(value); err != nil {
		return fmt.Errorf("can't encode %v: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(buffer.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
