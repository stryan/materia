package attributes

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ExtractVaultAttributes(t *testing.T) {
	tests := []struct {
		name   string
		start  map[string]any
		input  AttributeVault
		filter AttributesFilter
		want   map[string]any
	}{
		{
			name:  "test-host",
			start: map[string]any{},
			input: AttributeVault{
				Globals:    map[string]any{},
				Components: map[string]map[string]any{},
				Hosts: map[string]map[string]any{
					"test": {
						"key": "value",
					},
				},
				Roles: map[string]map[string]any{},
			},
			filter: AttributesFilter{
				Hostname: "test",
			},
			want: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "test-role",
			start: map[string]any{},
			input: AttributeVault{
				Globals: map[string]any{
					"key": "wrong-value",
				},
				Components: map[string]map[string]any{},
				Hosts:      map[string]map[string]any{},
				Roles: map[string]map[string]any{
					"test-role": {
						"key": "value",
					},
				},
			},
			filter: AttributesFilter{
				Hostname:  "",
				Roles:     []string{"test-role"},
				Component: "",
			},
			want: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "test-host-override-role",
			start: map[string]any{},
			input: AttributeVault{
				Globals:    map[string]any{},
				Components: map[string]map[string]any{},
				Hosts: map[string]map[string]any{
					"testhost": {
						"key": "value",
					},
				},
				Roles: map[string]map[string]any{
					"test-role": {
						"key": "wrong-value",
					},
				},
			},
			filter: AttributesFilter{
				Hostname:  "testhost",
				Roles:     []string{"test-role"},
				Component: "",
			},
			want: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "test-role-override-global",
			start: map[string]any{},
			input: AttributeVault{
				Globals: map[string]any{
					"key": "wrong-value",
				},
				Components: map[string]map[string]any{},
				Hosts:      map[string]map[string]any{},
				Roles: map[string]map[string]any{
					"test-role": {
						"key": "value",
					},
				},
			},
			filter: AttributesFilter{
				Hostname:  "",
				Roles:     []string{"test-role"},
				Component: "",
			},
			want: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "test-host-override-global",
			start: map[string]any{},
			input: AttributeVault{
				Globals:    map[string]any{},
				Components: map[string]map[string]any{},
				Hosts:      map[string]map[string]any{},
				Roles:      map[string]map[string]any{},
			},
			filter: AttributesFilter{
				Hostname:  "",
				Roles:     []string{},
				Component: "",
			},
			want: map[string]any{},
		},
		{
			name: "test-override-previous",
			start: map[string]any{
				"key": "value",
			},
			input: AttributeVault{
				Globals: map[string]any{
					"key": "wrong-value",
				},
				Components: map[string]map[string]any{},
				Hosts: map[string]map[string]any{
					"test": {
						"key": "value2",
					},
				},
				Roles: map[string]map[string]any{},
			},
			filter: AttributesFilter{
				Hostname:  "test",
				Roles:     []string{},
				Component: "",
			},
			want: map[string]any{
				"key": "value2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := maps.Clone(tt.start)
			ExtractVaultAttributes(input, tt.input, tt.filter)
			assert.Equal(t, input, tt.want)
		})
	}
}

func Test_SortedVaultFiles(t *testing.T) {
	tests := []struct {
		name          string
		filter        AttributesFilter
		vaultfiles    []string
		generalVaults []string
		want          []string
	}{
		{
			name: "empty inputs",
			filter: AttributesFilter{
				Hostname: "ivy",
				Roles:    []string{"testrole"},
			},
			vaultfiles:    []string{},
			generalVaults: []string{},
			want:          []string{},
		},
		{
			name: "only general vaults",
			filter: AttributesFilter{
				Hostname: "ivy",
				Roles:    []string{"testrole"},
			},
			vaultfiles: []string{
				"vault.yml",
				"attributes.yml",
			},
			generalVaults: []string{"vault.yml", "attributes.yml"},
			want: []string{
				"vault.yml",
				"attributes.yml",
			},
		},
		{
			name: "only role vaults",
			filter: AttributesFilter{
				Hostname: "",
				Roles:    []string{"testrole"},
			},
			vaultfiles: []string{
				"testrole.yml",
			},
			generalVaults: []string{},
			want: []string{
				"testrole.yml",
			},
		},
		{
			name: "only host vaults",
			filter: AttributesFilter{
				Hostname: "ivy",
				Roles:    []string{},
			},
			vaultfiles: []string{
				"ivy.yml",
			},
			generalVaults: []string{},
			want: []string{
				"ivy.yml",
			},
		},
		{
			name: "all three types in correct priority order",
			filter: AttributesFilter{
				Hostname: "ivy",
				Roles:    []string{"testrole"},
			},
			vaultfiles: []string{
				"vault.yml",
				"testrole.yml",
				"ivy.yml",
			},
			generalVaults: []string{"vault.yml"},
			want: []string{
				"vault.yml",
				"testrole.yml",
				"ivy.yml",
			},
		},
		{
			name: "files in mixed order get sorted correctly",
			filter: AttributesFilter{
				Hostname: "testhost",
				Roles:    []string{"base"},
			},
			vaultfiles: []string{
				"testhost.yml",
				"vault.yml",
				"base.yml",
				"attributes.yml",
			},
			generalVaults: []string{"vault.yml", "attributes.yml"},
			want: []string{
				"vault.yml",
				"attributes.yml",
				"base.yml",
				"testhost.yml",
			},
		},
		{
			name: "multiple roles",
			filter: AttributesFilter{
				Hostname: "yamato",
				Roles:    []string{"testrole", "base"},
			},
			vaultfiles: []string{
				"vault.yml",
				"testrole.yml",
				"base.yml",
			},
			generalVaults: []string{"vault.yml"},
			want: []string{
				"vault.yml",
				"testrole.yml",
				"base.yml",
			},
		},
		{
			name: "no matching files",
			filter: AttributesFilter{
				Hostname: "slork",
				Roles:    []string{"boho"},
			},
			vaultfiles: []string{
				"ivy.yml",
				"testrole.yml",
			},
			generalVaults: []string{},
			want:          []string{},
		},
		{
			name: "hostname-overrides-general-vault",
			filter: AttributesFilter{
				Hostname: "vault",
				Roles:    []string{"base"},
			},
			vaultfiles: []string{
				"vault.yml",
				"base.yml",
				"attributes.yml",
			},
			generalVaults: []string{"vault.yml", "attributes.yml"},
			want: []string{
				"attributes.yml",
				"base.yml",
				"vault.yml",
			},
		},
		{
			name: "hostname-overrides-rolename",
			filter: AttributesFilter{
				Hostname: "ivy",
				Roles:    []string{"ivy", "warden"},
			},
			vaultfiles: []string{
				"ivy.yml",
				"warden.yml",
			},
			generalVaults: []string{},
			want: []string{
				"warden.yml",
				"ivy.yml",
			},
		},
		{
			name: "rolename-overrides-general",
			filter: AttributesFilter{
				Hostname: "ivy",
				Roles:    []string{"vault"},
			},
			vaultfiles: []string{
				"ivy.yml",
				"vault.yml",
				"secrets.yml",
				"warden.yml",
			},
			generalVaults: []string{"vault.yml", "secrets.yml"},
			want: []string{
				"secrets.yml",
				"vault.yml",
				"ivy.yml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SortedVaultFiles(context.Background(), tt.filter, tt.vaultfiles, tt.generalVaults)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "wanted %v got %v", tt.want, got)
		})
	}
}
