package oci

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type OCISource struct {
	registry        string
	repository      string
	tag             string
	localRepository string
	auth            authn.Authenticator
	insecure        bool
}

func NewOCISource(c *Config) (*OCISource, error) {
	if c == nil {
		return nil, errors.New("need OCI config")
	}

	o := &OCISource{
		registry:        c.Registry,
		repository:      c.Repository,
		tag:             c.Tag,
		localRepository: c.LocalRepository,
		insecure:        c.Insecure,
	}

	if c.Username != "" && c.Password != "" {
		o.auth = &authn.Basic{
			Username: c.Username,
			Password: c.Password,
		}
	} else {
		o.auth = authn.Anonymous
	}

	return o, nil
}

func (o *OCISource) Sync(ctx context.Context) error {
	imageRef := fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag)
	log.Infof("Pulling OCI image %s", imageRef)

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	opts := []remote.Option{
		remote.WithAuth(o.auth),
		remote.WithContext(ctx),
	}

	if o.insecure {
		opts = append(opts, remote.WithTransport(remote.DefaultTransport))
	}

	img, err := remote.Image(ref, opts...)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	log.Debugf("Found %d layers in image", len(layers))

	if err := os.MkdirAll(o.localRepository, 0o755); err != nil {
		return fmt.Errorf("failed to create local repository: %w", err)
	}

	contentsTar := mutate.Extract(img)
	defer func() {
		_ = contentsTar.Close()
	}()

	tr := tar.NewReader(contentsTar)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(o.localRepository, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", target, err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}

			_, err = io.Copy(f, tr)
			_ = f.Close()
			if err != nil {
				return fmt.Errorf("failed to write file %s: %w", target, err)
			}
		default:
			log.Debugf("Skipping unsupported file type %c for %s", header.Typeflag, header.Name)
		}
	}

	log.Infof("Successfully extracted OCI image to %s", o.localRepository)
	return nil
}

func (o *OCISource) Close(ctx context.Context) error {
	// Nothing to close for OCI sources
	return nil
}

func (o *OCISource) Clean() error {
	return os.RemoveAll(o.localRepository)
}
