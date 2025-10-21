package attributes

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
