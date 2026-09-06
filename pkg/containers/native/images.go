package native

import (
	"context"
	"errors"

	"go.podman.io/podman/v6/pkg/bindings/images"
	"primamateria.systems/materia/pkg/containers"
)

func (n *NativeManager) GetImage(_ context.Context, nameOrId string) (*containers.Image, error) {
	img, err := images.GetImage(n.conn, nameOrId, nil)
	if err != nil {
		return nil, err
	}
	return &containers.Image{
		Names: img.RepoTags,
		ID:    img.ID,
	}, nil
}

func (n *NativeManager) ListImages(ctx context.Context) ([]*containers.Image, error) {
	imageList, err := images.List(n.conn, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*containers.Image, 0, len(imageList))
	for _, i := range imageList {
		result = append(result, &containers.Image{
			Names: i.RepoTags,
			ID:    i.ID,
		})
	}
	return result, nil
}

func (n *NativeManager) RemoveImage(_ context.Context, nameOrId string) error {
	_, err := images.Remove(n.conn, []string{nameOrId}, &images.RemoveOptions{})
	return errors.Join(err...)
}
