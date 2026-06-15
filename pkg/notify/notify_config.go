package notify

import (
	"fmt"

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

func NewConfig(k *koanf.Koanf) (*NotifyConfig, error) {
	var c NotifyConfig
	c.Triggers = k.StringMap("notify.triggers")

	return &c, nil
}

func (c *NotifyConfig) Validate() error {
	for k := range c.Triggers {
		if _, ok := notifyTypeMap[k]; !ok {
			return fmt.Errorf("unknown notify trigger type %v", k)
		}
	}
	return nil
}
