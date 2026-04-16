package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/pkg/manifests"
)

type TestComponent struct {
	Name   string
	Files  []TestFile
	Output []TestFile
}

type TestRepo struct {
	Manifest        *manifests.MateriaManifest
	Remote          bool
	Components      []TestComponent
	AttributesKind  string
	Attributes      map[string]attributes.AttributeVault
	Pubkey, Privkey string
}

type TestOutput struct {
	ActiveServices   []string
	InactiveServices []string
	Components       []string
	Files            []TestFile
}

type TestFile struct {
	Path    string
	IsDir   bool
	Content string
}

func (r *TestRepo) Write(ctx context.Context, basedir string) error {
	compdir := filepath.Join(basedir, "components")
	attrdir := filepath.Join(basedir, "attributes")
	if err := os.Mkdir(compdir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(attrdir, 0o755); err != nil {
		return err
	}
	if r.Manifest != nil {
		file, err := os.Create(filepath.Join(basedir, "MANIFEST.toml"))
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		err = toml.NewEncoder(file).Encode(r.Manifest)
		if err != nil {
			return err
		}
	}
	for _, c := range r.Components {
		if err := c.Write(compdir); err != nil {
			return err
		}
	}
	if r.Attributes != nil {
		for k, v := range r.Attributes {
			vaultfile := filepath.Join(attrdir, k)
			file, err := os.Create(vaultfile)
			if err != nil {
				return fmt.Errorf("unable to create file %s: %w", k, err)
			}
			defer func() {
				err := file.Close()
				if err != nil {
					log.Warn("error closing attr file: %v", err)
				}
			}()
			switch r.AttributesKind {
			case "file", "":
				err = toml.NewEncoder(file).Encode(v)
				if err != nil {
					return fmt.Errorf("failed to encode attr to TOML: %w", err)
				}
			case "sops":
				writer := yaml.NewEncoder(file)
				if err := writer.Encode(v); err != nil {
					cerr := writer.Close()
					if cerr != nil {
						return fmt.Errorf("failed to encode attr to yaml for sops: %w and %w", err, cerr)
					}
					return fmt.Errorf("failed to encode attr to yaml for sops: %w", err)
				}
				if err := sopsEncryptFile(ctx, r.Pubkey, vaultfile); err != nil {
					return fmt.Errorf("failed to sops encrypt file: %w", err)
				}
				if err := os.Remove(vaultfile); err != nil {
					return fmt.Errorf("failed to remove source sops file: %w", err)
				}
			default:
				return fmt.Errorf("invalid attributes kind: %v", r.AttributesKind)
			}
		}
	}
	return nil
}

func (c *TestComponent) Write(basedir string) error {
	compdir := filepath.Join(basedir, c.Name)
	if err := os.Mkdir(compdir, 0o755); err != nil {
		return err
	}
	for _, f := range c.Files {
		if err := f.Write(compdir); err != nil {
			return err
		}
	}
	return nil
}

func (o *TestOutput) Write(basedir string) error {
	for _, f := range o.Files {
		if err := f.Write(basedir); err != nil {
			return err
		}
	}

	return nil
}

func (f *TestFile) Write(basedir string) error {
	output := filepath.Join(basedir, f.Path)
	err := os.MkdirAll(filepath.Dir(output), 0o755)
	if err != nil {
		return err
	}
	if f.IsDir {
		return os.Mkdir(output, 0o755)
	}
	return os.WriteFile(output, []byte(f.Content), 0o755)
}

func injectHostAttribute(vault attributes.AttributeVault, host, key, value string) {
	if cur, ok := vault.Hosts[host]; !ok {
		vault.Hosts[host] = map[string]any{
			key: value,
		}
	} else {
		cur[key] = value
		vault.Hosts[host] = cur
	}
}

func injectRoleAttribute(vault attributes.AttributeVault, role, key, value string) {
	if cur, ok := vault.Roles[role]; !ok {
		vault.Roles[role] = map[string]any{
			key: value,
		}
	} else {
		cur[key] = value
		vault.Roles[role] = cur
	}
}

func injectComponentAttribute(vault attributes.AttributeVault, comp, key, value string) {
	if cur, ok := vault.Components[comp]; !ok {
		vault.Components[comp] = map[string]any{
			key: value,
		}
	} else {
		cur[key] = value
		vault.Components[comp] = cur
	}
}
