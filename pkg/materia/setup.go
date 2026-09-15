package materia

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/attributes/age"
	fileattrs "primamateria.systems/materia/pkg/attributes/file"
	"primamateria.systems/materia/pkg/attributes/mem"
	"primamateria.systems/materia/pkg/attributes/sops"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/source"
	"primamateria.systems/materia/pkg/sourceman"
)

func NewFromKoanf(ctx context.Context, k *koanf.Koanf, src source.Source) (*Materia, error) {
	c, err := NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	return NewFromConfig(ctx, c, src)
}

func NewFromConfig(ctx context.Context, c *MateriaConfig, src source.Source) (*Materia, error) {
	if err := SetupDirectories(c); err != nil {
		return nil, fmt.Errorf("error creating base directories: %w", err)
	}
	hmc := &hostman.HostmanConfig{
		Hostname:         c.Hostname,
		DataDir:          c.MateriaDir,
		QuadletDir:       c.QuadletDir,
		ScriptsDir:       c.ScriptsDir,
		ServicesDir:      c.ServiceDir,
		ContainersConfig: c.ContainersConfig,
		ServicesConfig:   c.ServicesConfig,
	}
	smc := &sourceman.SourceManConfig{
		SourceDir: c.SourceDir,
		RemoteDir: c.RemoteDir,
	}

	hm, err := hostman.NewHostManager(ctx, hmc)
	if err != nil {
		return nil, err
	}

	sm, err := sourceman.NewSourceManager(smc)
	if err != nil {
		return nil, err
	}
	if err := sm.AddSource(src, nil, nil, true); err != nil {
		return nil, err
	}
	if !c.NoSync {
		if err := sm.Sync(ctx, nil); err != nil {
			return nil, fmt.Errorf("error with initial repo sync: %w", err)
		}
		if err := sm.LoadRemotes(ctx); err != nil {
			return nil, fmt.Errorf("error with repo remotes load: %w", err)
		}
	}
	vault, err := setupVault(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes engine: %w", err)
	}

	return New(ctx, c, hm, sm, vault)
}

func setupVault(c *MateriaConfig) (AttributesEngine, error) {
	var vaults []AttributesEngine
	if c.AgeConfig != nil {
		vault, err := age.NewAgeStore(*c.AgeConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}
		if c.Attributes == "age" {
			return vault, nil
		}
		vaults = append(vaults, vault)
	}
	if c.FileConfig != nil {
		vault, err := fileattrs.NewFileStore(*c.FileConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating file store: %w", err)
		}

		if c.Attributes == "file" {
			return vault, nil
		}
		vaults = append(vaults, vault)
	}
	if c.SopsConfig != nil {
		vault, err := sops.NewSopsStore(*c.SopsConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating sops store: %w", err)
		}
		if c.Attributes == "sops" {
			return vault, nil
		}

		vaults = append(vaults, vault)
	}
	if len(vaults) == 0 {
		log.Warn("No attributes engines configured: defaulting to in-memory")
		return mem.NewMemoryEngine(), nil
	}
	return NewMultiVaultEngine(vaults...)
}

func SetupLogger(c *MateriaConfig) {
	if c.UseStdout {
		log.Default().SetOutput(os.Stdout)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
}

func SetupDirectories(c *MateriaConfig) error {
	err := os.Mkdir(c.MateriaDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(c.OutputDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating output dir: %w", err)
	}
	err = os.Mkdir(c.SourceDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.MkdirAll(filepath.Join(c.RemoteDir, "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(c.MateriaDir, "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating components in prefix: %w", err)
	}
	return nil
}
