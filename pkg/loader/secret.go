package loader

import (
	"context"
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/pkg/components"
)

type SecretInjectorStage struct {
	attrs map[string]any
}

func (s *SecretInjectorStage) Process(ctx context.Context, comp *components.Component) error {
	for _, r := range comp.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			newSecret, ok := s.attrs[r.Path]
			if !ok {
				newSecret = ""
			}
			newSecretString, isString := newSecret.(string)
			if !isString {
				return fmt.Errorf("tried to load a non-string for secret %v", r.Path)
			}
			r.Content = newSecretString
			comp.Resources.Set(r)
		}
	}
	return nil
}

type SecretDiscoveryStage struct {
	manager containers.ContainerManager
}

func (s *SecretDiscoveryStage) Process(ctx context.Context, comp *components.Component) error {
	var deletedSecrets []components.Resource
	for _, r := range comp.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			secretsList, err := s.manager.ListSecrets(ctx)
			if err != nil {
				return fmt.Errorf("error listing secrets during resource validation")
			}
			if !slices.Contains(secretsList, r.Path) {
				// secret isn't there so we treat it like the resource never existed
				deletedSecrets = append(deletedSecrets, r)
			} else {
				curSecret, err := s.manager.GetSecret(ctx, r.Path)
				if err != nil {
					return err
				}
				r.Content = curSecret.Value
				comp.Resources.Set(r)
			}
		}
	}
	for _, r := range deletedSecrets {
		comp.Resources.Delete(r.Path)
	}
	return nil
}
