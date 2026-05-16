package command

import (
	"context"
	"encoding/json"
	"fmt"

	"primamateria.systems/materia/pkg/containers"
)

func (p *CommandManager) ListImages(ctx context.Context) ([]*containers.Image, error) {
	cmd := genCmd(ctx, p.remote, "image", "ls", "--format", "json")
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error listing podman images: %w", err)
	}
	var images []*containers.Image
	if err := json.Unmarshal(output.Bytes(), &images); err != nil {
		return nil, err
	}
	return images, nil
}

func (p *CommandManager) RemoveImage(ctx context.Context, name string) error {
	cmd := genCmd(ctx, p.remote, "image", "rm", name)
	_, err := runCmd(cmd)
	if err != nil {
		return err
	}
	return nil
}
