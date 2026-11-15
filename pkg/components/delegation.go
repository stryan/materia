package components

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"

	"primamateria.systems/materia/pkg/manifests"
)

var userNameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// ValidateUser checks an account name from a component's User setting.
// The name is used as a directory name, so it must not hold path separators.
func ValidateUser(name string) error {
	if name == "root" {
		return errors.New("invalid User \"root\": leave User unset to install as root")
	}
	if !userNameRegex.MatchString(name) {
		return fmt.Errorf("invalid User %q: need a system account name", name)
	}
	return nil
}

// Delegation holds everything an account's own materia needs to install a
// component: the component's files as they are in the source, the manifest
// with host overrides and extensions applied, and the attributes materia
// resolved for it.
//
// Templates are not rendered. The account's materia renders them, so macros
// such as m_dataDir resolve to the account's directories.
type Delegation struct {
	User       string
	Manifest   []byte
	Files      []DelegatedFile
	Attributes map[string]any
	Repository DelegatedRepository
}

// DelegatedFile is one file or directory of a delegated component.
// Path is the path in the source, so a template keeps its .gotmpl suffix.
type DelegatedFile struct {
	Path    string
	Mode    fs.FileMode
	Dir     bool
	Content string
}

// DelegatedRepository holds the parts of the repository manifest that the
// account's materia needs to render the component the way root would.
type DelegatedRepository struct {
	Hostname string
	Roles    []string
	Snippets []manifests.SnippetConfig
}

// Digest identifies the staged content. It changes when anything the
// account's materia would read changes.
func (d *Delegation) Digest() (string, error) {
	if d == nil {
		return "", errors.New("no delegation")
	}
	// encoding/json sorts map keys, so the encoding is stable
	data, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("can't encode delegation: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
