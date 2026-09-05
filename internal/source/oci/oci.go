package oci

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"primamateria.systems/materia/pkg/source"
)

var revName = ".materia_revision"

type OCISource struct {
	registry        string
	repository      string
	tag             string
	localRepository string
	auth            authn.Authenticator
	insecure        bool

	lastDigest  string
	canRollback bool
}

func NewOCISource(c *Config) (*OCISource, error) {
	if c == nil {
		return nil, errors.New("need OCI config")
	}
	if err := c.parseURL(); err != nil {
		return nil, fmt.Errorf("unable to parse OCI url: %w", err)
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

	if _, err := os.Stat(filepath.Join(c.LocalRepository, revName)); err == nil {
		revData, err := os.ReadFile(filepath.Join(c.LocalRepository, revName))
		if err != nil {
			log.Warnf("unable to read existing revision for rollback: %v", err)
		} else {
			o.lastDigest = string(revData)
			o.canRollback = true
		}
	}

	return o, nil
}

func (o *OCISource) Sync(ctx context.Context, opts source.SyncOpts) (*source.SyncReport, error) {
	revision := fmt.Sprintf(":%v", o.tag)
	if opts.Revision != "" {
		if strings.Contains(opts.Revision, "sha256") {
			// we're pulling a specific digest not just a tag
			revision = fmt.Sprintf("@%v", opts.Revision)
		} else {
			revision = fmt.Sprintf(":%v", opts.Revision)
		}
	}
	currentDigest := o.lastDigest
	imageRef := fmt.Sprintf("%s/%s%s", o.registry, o.repository, revision)
	log.Infof("Pulling OCI image %s", imageRef)

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	remoteOpts := []remote.Option{
		remote.WithAuth(o.auth),
		remote.WithContext(ctx),
	}

	if o.insecure {
		remoteOpts = append(remoteOpts, remote.WithTransport(remote.DefaultTransport))
	}

	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get image layers: %w", err)
	}

	log.Debugf("Found %d layers in image", len(layers))

	if err := os.MkdirAll(o.localRepository, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create local repository: %w", err)
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
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(o.localRepository, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, fmt.Errorf("failed to create parent directory for %s: %w", target, err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", target, err)
			}

			_, err = io.Copy(f, tr)
			_ = f.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to write file %s: %w", target, err)
			}
		default:
			log.Debugf("Skipping unsupported file type %c for %s", header.Typeflag, header.Name)
		}
	}
	digest, err := img.Digest()
	if err != nil {
		log.Warnf("Unable to fetch image digest for rollback support")
	} else {
		err = os.WriteFile(filepath.Join(o.localRepository, revName), []byte(digest.String()), 0o644)
		if err != nil {
			log.Warnf("Unable to save image digest for rollback support")
			o.canRollback = false
			err := os.Remove(filepath.Join(o.localRepository, revName))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("unable to remove stale OCI revision state: %w", err)
			}
		} else {
			o.lastDigest = digest.String()
			o.canRollback = true
		}
	}
	log.Infof("Successfully extracted OCI image to %s", o.localRepository)
	return &source.SyncReport{
		OldRevision: currentDigest,
		NewRevision: o.lastDigest,
	}, nil
}

func (o *OCISource) Close(ctx context.Context) error {
	// Nothing to close for OCI sources
	return nil
}

func (o *OCISource) Clean() error {
	return os.RemoveAll(o.localRepository)
}

func (o *OCISource) Inspect() source.SyncInspectReport {
	return source.SyncInspectReport{
		SupportsRollback: o.canRollback,
	}
}

func (o *OCISource) String() string {
	imageRef := fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag)
	return fmt.Sprintf("oci:%v", imageRef)
}
