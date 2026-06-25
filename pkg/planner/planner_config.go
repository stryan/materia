package planner

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type PlannerConfig struct {
	OnlyResources   bool `koanf:"only_resources"`
	CleanupQuadlets bool `koanf:"cleanup_quadlets"`
	CleanupVolumes  bool `koanf:"cleanup_volumes"`
	BackupVolumes   bool `koanf:"backup_volumes"`
	MigrateVolumes  bool `koanf:"migrate_volumes"`
}

func NewPlannerConfig(k *koanf.Koanf) (*PlannerConfig, error) {
	pc := DefaultPlannerConfig()
	err := k.UnmarshalWithConf("planner", pc, koanf.UnmarshalConf{})
	if err != nil {
		return nil, fmt.Errorf("unable to create planner config: %w", err)
	}

	return pc, nil
}

func DefaultPlannerConfig() *PlannerConfig {
	return &PlannerConfig{
		OnlyResources:   false,
		CleanupQuadlets: false,
		CleanupVolumes:  false,
		BackupVolumes:   true,
		MigrateVolumes:  false,
	}
}

func (p *PlannerConfig) String() string {
	return fmt.Sprintf("Cleanup Quadlets: %v\nCleanup Volumes: %v\nBackup Volumes: %v\nMigrate Volumes: %v\n", p.CleanupQuadlets, p.CleanupVolumes, p.BackupVolumes, p.MigrateVolumes)
}

func (p *PlannerConfig) Validate() error {
	return nil
}
