package planner

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type PlannerConfig struct {
	OnlyResources   bool `toml:"only_resources"`
	CleanupQuadlets bool `toml:"cleanup_quadlets"`
	CleanupVolumes  bool `toml:"cleanup_volumes"`
	BackupVolumes   bool `toml:"backup_volumes"`
	MigrateVolumes  bool `toml:"migrate_volumes"`
}

func NewPlannerConfig(k *koanf.Koanf) (*PlannerConfig, error) {
	pc := &PlannerConfig{}
	pc.CleanupQuadlets = k.Bool("planner.cleanup_quadlets")
	pc.CleanupVolumes = k.Bool("planner.cleanup_volumes")
	if k.Exists("planner.backup_volumes") {
		pc.BackupVolumes = k.Bool("planner.backup_volumes")
	} else {
		pc.BackupVolumes = true
	}
	pc.MigrateVolumes = k.Bool("planner.migrate_volumes")

	return pc, nil
}

func (p *PlannerConfig) String() string {
	return fmt.Sprintf("Cleanup Quadlets: %v\nCleanup Volumes: %v\nBackup Volumes: %v\nMigrate Volumes: %v\n", p.CleanupQuadlets, p.CleanupVolumes, p.BackupVolumes, p.MigrateVolumes)
}

func (p *PlannerConfig) Validate() error {
	return nil
}
