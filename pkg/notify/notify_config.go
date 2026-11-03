package notify

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
)

type NotifyConfig struct {
	Triggers map[string]string `koanf:"triggers" toml:"triggers"`
}

const (
	NotifyUnknown  = "unknown"
	NotifyDefault  = "default"
	NotifyUpdate   = "update"
	NotifyRollback = "rollback"
)

var notifyTypeMap = map[string]string{
	"unknown":  NotifyUnknown,
	"default":  NotifyDefault,
	"update":   NotifyUpdate,
	"rollback": NotifyRollback,
}

func NewNotifyType(name string) string {
	if res, ok := notifyTypeMap[name]; ok {
		return res
	} else {
		return NotifyUnknown
	}
}

func NewConfig(k *koanf.Koanf) (NotifyConfig, error) {
	nc := DefaultNotifyConfig()
	if err := k.UnmarshalWithConf("notify", &nc, koanf.UnmarshalConf{}); err != nil {
		return NotifyConfig{}, fmt.Errorf("unable to create notify config: %w", err)
	}
	return nc, nil
}

func (c NotifyConfig) Validate() error {
	for k := range c.Triggers {
		if _, ok := notifyTypeMap[k]; !ok {
			return fmt.Errorf("unknown notify trigger type %v", k)
		}
	}
	return nil
}

func DefaultNotifyConfig() NotifyConfig {
	return NotifyConfig{
		Triggers: make(map[string]string),
	}
}

func (c NotifyConfig) String() string {
	var sb strings.Builder
	for k, v := range c.Triggers {
		fmt.Fprintf(&sb, "%v: %v\n", k, v)
	}
	return sb.String()
}
