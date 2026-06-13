package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"charm.land/log/v2"
	"filippo.io/age"
	koanftoml "github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/v2"
	"github.com/testcontainers/testcontainers-go"
)

type TestCase struct {
	Name            string
	Config          *koanf.Koanf
	Source          TestRepo
	Output          TestOutput
	Pubkey, Privkey string
}

func (c *TestCase) Destination() string {
	return filepath.Join("/root/tests/", c.Name)
}

func (c *TestCase) Setup() error {
	if c.Source.AttributesKind == "age" || c.Source.AttributesKind == "sops" {
		identity, err := age.GenerateX25519Identity()
		if err != nil {
			return err
		}
		c.Pubkey = identity.Recipient().String()
		c.Privkey = identity.String()
		c.Source.Pubkey = c.Pubkey
		c.Source.Privkey = c.Privkey
	}
	return nil
}

func (c *TestCase) Write(ctx context.Context, base string) error {
	baseDir := filepath.Join(base, c.Name)
	if err := os.Mkdir(baseDir, 0o755); err != nil {
		return err
	}
	sourceDir := filepath.Join(baseDir, "source")
	configDir := filepath.Join(baseDir, "config")
	outputDir := filepath.Join(baseDir, "output")

	if err := os.Mkdir(sourceDir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(configDir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		return err
	}
	if c.Config != nil {
		data, err := c.Config.Marshal(koanftoml.Parser())
		if err != nil {
			return err
		}

		file, err := os.Create(filepath.Join(configDir, "config.toml"))
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		_, err = file.Write(data)
		if err != nil {
			return err
		}
	}
	if c.Privkey != "" {
		file, err := os.Create(filepath.Join(configDir, "key.txt"))
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		data := fmt.Sprintf("#created: present-day,present-time\n# public key: %v\n%v\n", c.Pubkey, c.Privkey) // HAHAHAHAHA
		_, err = file.Write([]byte(data))
		if err != nil {
			return err
		}
	}
	if !c.Source.Remote {
		if err := c.Source.Write(ctx, sourceDir); err != nil {
			return err
		}
	}

	if err := c.Output.Write(outputDir); err != nil {
		return err
	}

	return nil
}

func installTestCase(ctx context.Context, c testcontainers.Container, tcs ...TestCase) error {
	baseDir, err := os.MkdirTemp("", "materia-test")
	if err != nil {
		return err
	}
	testDir := filepath.Join(baseDir, "tests")
	err = os.Mkdir(testDir, 0o755)
	if err != nil {
		return err
	}
	for _, v := range tcs {
		err = v.Write(ctx, testDir)
		if err != nil {
			return err
		}
	}
	defer func() {
		_ = os.RemoveAll(baseDir)
	}()
	if err := copyDirToContainer(ctx, c, testDir, "/root/"); err != nil {
		return err
	}
	return nil
}

func checkTestCase(ctx context.Context, c testcontainers.Container, tc TestCase) error {
	installed, err := listInstalledComponents(ctx, c)
	if err != nil {
		return err
	}
	for _, c := range tc.Output.Components {
		if !slices.Contains(installed, c) {
			return fmt.Errorf("missing component %v: have %v", c, installed)
		}
	}
	for _, c := range installed {
		if !slices.Contains(tc.Output.Components, c) {
			return fmt.Errorf("extra component: %v", c)
		}
	}
	for _, f := range tc.Output.Files {
		diff, err := compareFile(ctx, c, filepath.Join("/root/tests", tc.Name, "output", f.Path), f.Path)
		if err != nil {
			return fmt.Errorf("%w: %v", err, diff)
		}
	}
	for _, s := range tc.Output.ActiveServices {
		// TODO This doesn't seem to work right
		attempts := 0
		if !getService(ctx, c, s, "active") && attempts > 5 {
			return fmt.Errorf("inactive service: %v", s)
		} else {
			log.Infof("waiting on %v to start", s)
			attempts++
			time.Sleep(2 * time.Second)
		}
	}
	for _, s := range tc.Output.InactiveServices {
		attempts := 0
		if !getService(ctx, c, s, "inactive") && attempts > 5 {
			return fmt.Errorf("inactive service: %v", s)
		} else {
			log.Infof("waiting on %v to stop", s)
			attempts++
			time.Sleep(2 * time.Second)
		}
	}

	return nil
}
