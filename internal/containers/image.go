package containers

import (
	"context"
	"encoding/json"
	"fmt"
)

type Image struct {
	Names []string `json:"Names"`
	ID    string   `json:"ID"`
}

func (p *PodmanManager) ListImages(ctx context.Context) ([]*Image, error) {
	cmd := genCmd(ctx, p.remote, "image", "ls", "--format", "json")
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error listing podman images: %w", err)
	}
	var images []*Image
	if err := json.Unmarshal(output.Bytes(), &images); err != nil {
		return nil, err
	}
	return images, nil
}

func (p *PodmanManager) RemoveImage(ctx context.Context, name string) error {
	cmd := genCmd(ctx, p.remote, "image", "rm", name)
	_, err := runCmd(cmd)
	if err != nil {
		return err
	}
	return nil
}
