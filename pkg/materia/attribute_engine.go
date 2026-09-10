package materia

import (
	"context"
	"errors"

	"primamateria.systems/materia/internal/attributes"
)

type AttributesEngine interface {
	Lookup(context.Context, attributes.AttributesFilter) (map[string]any, error)
}

type MultiVaultEngine struct {
	vaults []AttributesEngine
}

func NewMultiVaultEngine(vaults ...AttributesEngine) (*MultiVaultEngine, error) {
	if len(vaults) < 1 {
		return nil, errors.New("need vaults for multivault engine")
	}
	return &MultiVaultEngine{
		vaults: vaults,
	}, nil
}

func (m *MultiVaultEngine) AddVault(vault AttributesEngine) error {
	if vault == nil {
		return errors.New("need vault")
	}
	m.vaults = append(m.vaults, vault)
	return nil
}

func (m *MultiVaultEngine) Lookup(ctx context.Context, filter attributes.AttributesFilter) (map[string]any, error) {
	results := make(map[string]any)
	// TODO there's definitely a better way of doing this and we'll burn that bridge when we get to it
	for _, v := range m.vaults {
		vaultResult, err := v.Lookup(ctx, filter)
		if err != nil {
			return nil, err
		}
		results = attributes.MergeAttributes(results, vaultResult)
	}
	return results, nil
}
