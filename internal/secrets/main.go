package secrets

type SecretsConfig interface {
	SecretsType() string
	Validate() error
	String() string
}

type Secret struct {
	Key, Value      string
	ContainerSecret bool
}

type SecretFilter struct {
	Hostname  string
	Roles     []string
	Component string
}

type SecretsVault struct {
	Globals    map[string]any
	Components map[string]map[string]any
	Hosts      map[string]map[string]any
	Roles      map[string]map[string]any
}

func MergeSecrets(higher map[string]any, lower map[string]any) map[string]any {
	for k, v := range lower {
		if _, ok := higher[k]; !ok {
			higher[k] = v
		}
	}
	return higher
}
