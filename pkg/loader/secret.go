package loader

import (
	"context"
	"fmt"
	"slices"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
)

type SecretInjectorStage struct {
	attrs map[string]any
}

type SecretsManager interface {
	ListSecrets(context.Context) ([]string, error)
	GetSecret(context.Context, string) (*containers.PodmanSecret, error)
}

func (s *SecretInjectorStage) Process(ctx context.Context, comp *components.Component) error {
	for _, r := range comp.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			newSecret, ok := s.attrs[r.Path]
			if !ok {
				// no attribute, no secret
				// not an error since some attributes may be conditional
				comp.Resources.Delete(r.Path)
				continue
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
	manager SecretsManager
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
