package attributes

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

type AttributesFilter struct {
	Hostname  string
	Roles     []string
	Component string
}

type AttributeVault struct {
	Globals    map[string]any            `toml:"globals" yaml:"globals" json:"globals" ini:"globals"`
	Components map[string]map[string]any `toml:"components" yaml:"components" json:"components" ini:"components"`
	Hosts      map[string]map[string]any `toml:"hosts" yaml:"hosts" json:"hosts" ini:"hosts"`
	Roles      map[string]map[string]any `toml:"roles" yaml:"roles" json:"roles" ini:"roles"`
}

func MergeAttributes(higher map[string]any, lower map[string]any) map[string]any {
	for k, v := range lower {
		if _, ok := higher[k]; !ok {
			higher[k] = v
		}
	}
	return higher
}

func ExtractVaultAttributes(results map[string]any, vault AttributeVault, filter AttributesFilter) {
	maps.Copy(results, vault.Globals)

	for _, role := range filter.Roles {
		if attrs, ok := vault.Roles[role]; ok {
			maps.Copy(results, attrs)
		}
	}

	if filter.Component != "" {
		if attrs, ok := vault.Components[filter.Component]; ok {
			maps.Copy(results, attrs)
		}
	}

	if filter.Hostname != "" {
		if attrs, ok := vault.Hosts[filter.Hostname]; ok {
			maps.Copy(results, attrs)
		}
	}
}

func SortedVaultFiles(ctx context.Context, f AttributesFilter, vaultfiles, generalVaults []string) ([]string, error) {
	var hostFiles, roleFiles, generalFiles []string
	for _, v := range vaultfiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if strings.Contains(v, f.Hostname) {
			hostFiles = append(hostFiles, v)
			continue
		}
		hasRole := false
		for _, r := range f.Roles {
			if strings.Contains(v, r) {
				roleFiles = append(roleFiles, v)
				hasRole = true
			}
		}
		if hasRole {
			continue
		}
		if slices.Contains(generalVaults, filepath.Base(v)) {
			generalFiles = append(generalFiles, v)
			continue
		}

	}
	// file list is in order of General Vaults, Role Vaults, Host Vaults
	// So host file keys override role keys override general keys
	files := make([]string, 0, len(hostFiles)+len(roleFiles)+len(generalFiles))
	files = append(files, generalFiles...)
	files = append(files, roleFiles...)
	files = append(files, hostFiles...)
	return files, nil
}
