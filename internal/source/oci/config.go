package oci

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	URL             string `toml:"url" json:"url" yaml:"url"`
	Tag             string `toml:"tag" json:"tag" yaml:"tag"`
	Username        string `toml:"username" json:"username" yaml:"username"`
	Password        string `toml:"password" json:"password" yaml:"password"`
	Insecure        bool   `toml:"insecure" json:"insecure" yaml:"insecure"`
	LocalRepository string `toml:"local_repository" json:"local_repository" yaml:"local_repository"`

	// Parsed fields
	Registry   string
	Repository string
}

func NewConfig(k *koanf.Koanf, localDir, remoteURL string) (*Config, error) {
	var c Config

	c.Username = k.String("oci.username")
	c.Password = k.String("oci.password")
	c.Insecure = k.Bool("oci.insecure")
	c.Tag = k.String("oci.tag")
	c.LocalRepository = localDir
	c.URL = remoteURL

	if err := c.parseURL(); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Config) parseURL() error {
	// Expected format: oci://registry.example.com/namespace/repository:tag
	// or: oci://registry.example.com/namespace/repository@sha256:digest
	url := c.URL

	url = strings.TrimPrefix(url, "oci://")

	parts := strings.SplitN(url, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid OCI URL format: expected oci://registry/repository[:tag|@digest]")
	}

	c.Registry = parts[0]
	repoAndTag := parts[1]

	// Check for tag or digest
	if strings.Contains(repoAndTag, "@") {
		// Digest format should be: repository@sha256:hash
		repoParts := strings.SplitN(repoAndTag, "@", 2)
		c.Repository = repoParts[0]
		c.Tag = repoParts[1] // This will be the digest
	} else if strings.Contains(repoAndTag, ":") {
		// Tag format should be: repository:tag
		repoParts := strings.SplitN(repoAndTag, ":", 2)
		c.Repository = repoParts[0]
		if c.Tag == "" {
			c.Tag = repoParts[1]
		}
	} else {
		// No tag or digest specified, just use latest
		c.Repository = repoAndTag
		if c.Tag == "" {
			c.Tag = "latest"
		}
	}

	return nil
}

func (c *Config) String() string {
	var result string
	result += fmt.Sprintf("Registry: %v\n", c.Registry)
	result += fmt.Sprintf("Repository: %v\n", c.Repository)
	result += fmt.Sprintf("Tag: %v\n", c.Tag)
	result += fmt.Sprintf("Allow Insecure: %v\n", c.Insecure)
	if c.Username != "" {
		result += fmt.Sprintf("Username: %v\n", c.Username)
	}
	return result
}
