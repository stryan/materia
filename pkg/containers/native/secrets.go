package native

import (
	"context"
	"fmt"
	"strings"

	sm "go.podman.io/podman/v6/pkg/bindings/secrets"
	"primamateria.systems/materia/pkg/containers"
)

func (n *NativeManager) GetSecret(_ context.Context, name string) (*containers.PodmanSecret, error) {
	sname := n.SecretName(name)
	getSecret := true // api won't return secret data by default
	secret, err := sm.Inspect(n.conn, sname, &sm.InspectOptions{
		ShowSecret: &getSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get secret %v: %w", name, err)
	}
	return &containers.PodmanSecret{
		Name:  name,
		Value: secret.SecretData,
	}, nil
}

func (n *NativeManager) ListSecrets(_ context.Context) ([]string, error) {
	secrets, err := sm.List(n.conn, &sm.ListOptions{
		Filters: map[string][]string{
			"name": {n.secretsPrefix},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list secrets: %w", err)
	}
	result := make([]string, 0, len(secrets))
	for _, s := range secrets {
		result = append(result, strings.TrimPrefix(s.Spec.Name, n.secretsPrefix))
	}
	return result, nil
}

func (n *NativeManager) RemoveSecret(_ context.Context, name string) error {
	return sm.Remove(n.conn, n.SecretName(name))
}

func (n *NativeManager) WriteSecret(_ context.Context, name, value string) error {
	sname := n.SecretName(name)
	replace := true
	_, err := sm.Create(n.conn, strings.NewReader(value), &sm.CreateOptions{
		Name:    &sname,
		Replace: &replace,
	})
	if err != nil {
		return fmt.Errorf("unable to write secret %v: %w", name, err)
	}
	return nil
}

func (n *NativeManager) SecretName(name string) string {
	return fmt.Sprintf("%v%v", n.secretsPrefix, name)
}
