package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/secrets"
	"primamateria.systems/materia/internal/services"
)

func (m *Materia) Execute(ctx context.Context, plan *Plan) (int, error) {
	if plan.Empty() {
		return -1, nil
	}
	defer func() {
		if !m.cleanup {
			return
		}
		problems, err := m.ValidateComponents(ctx)
		if err != nil {
			log.Warnf("error cleaning up execution: %v", err)
		}
		for _, v := range problems {
			log.Infof("component %v failed to install, purging", v)
			err := m.CompRepo.PurgeComponentByName(v)
			if err != nil {
				log.Warnf("error purging component: %v", err)
			}
		}
	}()
	serviceActions := []Action{}
	steps := 0
	// Template and install resources
	for _, v := range plan.Steps() {
		vars := make(map[string]any)
		if err := v.Validate(); err != nil {
			return steps, err
		}
		vaultVars := m.Secrets.Lookup(ctx, secrets.SecretFilter{
			Hostname:  m.HostFacts.GetHostname(),
			Roles:     m.Roles,
			Component: v.Parent.Name,
		})
		maps.Copy(vars, v.Parent.Defaults)
		maps.Copy(vars, vaultVars)
		err := m.executeAction(ctx, v, vars)
		if err != nil {
			return steps, err
		}

		if v.Todo == ActionStartService || v.Todo == ActionStopService || v.Todo == ActionRestartService || v.Todo == ActionEnableService || v.Todo == ActionDisableService {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	activating := []string{}
	deactivating := []string{}
	for _, v := range serviceActions {
		serv, err := m.Services.Get(ctx, v.Payload.Name)
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case ActionRestartService, ActionStartService:
			if serv.State == "activating" {
				activating = append(activating, v.Payload.Name)
			} else if serv.State != "active" {
				log.Warn("service failed to start/restart", "service", serv.Name, "state", serv.State)
			}
		case ActionStopService:
			if serv.State == "deactivating" {
				deactivating = append(deactivating, v.Payload.Name)
			} else if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		case ActionEnableService, ActionDisableService:
		default:
			return steps, errors.New("unknown service action state")
		}
	}
	var servWG sync.WaitGroup
	for _, v := range activating {
		servWG.Add(1)
		go func() {
			defer servWG.Done()
			err := m.Services.WaitUntilState(ctx, v, "active")
			if err != nil {
				log.Warn(err)
			}
		}()
	}
	for _, v := range deactivating {
		servWG.Add(1)
		go func() {
			defer servWG.Done()
			err := m.Services.WaitUntilState(ctx, v, "inactive")
			if err != nil {
				log.Warn(err)
			}
		}()
	}
	servWG.Wait()
	return steps, nil
}

func (m *Materia) executeAction(ctx context.Context, v Action, vars map[string]any) error {
	switch v.Todo {
	case ActionInstallComponent:
		if err := m.CompRepo.InstallComponent(v.Parent); err != nil {
			return err
		}
	case ActionUpdateComponent:
		if err := m.CompRepo.UpdateComponent(v.Parent); err != nil {
			return err
		}
	case ActionInstallFile, ActionUpdateFile, ActionInstallQuadlet, ActionUpdateQuadlet:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
	case ActionInstallScript, ActionUpdateScript:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.ScriptRepo.Install(ctx, v.Payload.Name, resourceData); err != nil {
			return err
		}
	case ActionInstallDirectory:
		if err := m.CompRepo.InstallResource(v.Payload, nil); err != nil {
			return err
		}
	case ActionInstallPodmanSecret, ActionUpdatePodmanSecret:
		var secretVar any
		var ok bool
		if secretVar, ok = vars[v.Payload.Name]; !ok {
			return errors.New("can't install/update Podman Secret: no matching Materia secret")
		}
		if value, ok := secretVar.(string); !ok {
			return errors.New("can't install/update Podman Secret: materia secret isn't string")
		} else {
			if err := m.Containers.WriteSecret(ctx, v.Payload.Name, value); err != nil {
				return err
			}
		}
	case ActionRemovePodmanSecret:
		if err := m.Containers.RemoveSecret(ctx, v.Payload.Name); err != nil {
			return err
		}
	case ActionInstallService, ActionUpdateService:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.ServiceRepo.Install(ctx, v.Payload.Name, resourceData); err != nil {
			return err
		}
	case ActionInstallComponentScript, ActionUpdateComponentScript:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
	case ActionRemoveFile:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveQuadlet:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveScript:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
		if err := m.ScriptRepo.Remove(ctx, v.Payload.Name); err != nil {
			return err
		}
	case ActionRemoveDirectory:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveService:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
		if err := m.ServiceRepo.Remove(ctx, v.Payload.Name); err != nil {
			return err
		}
	case ActionRemoveComponentScript:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveComponent:
		if err := m.CompRepo.RemoveComponent(v.Parent); err != nil {
			return err
		}
	case ActionCleanupComponent:
		if err := m.CompRepo.RunCleanup(v.Parent); err != nil {
			return err
		}
	case ActionEnsureVolume:
		service := strings.TrimSuffix(v.Payload.Name, ".volume")
		err := m.modifyService(ctx, Action{
			Todo:   ActionStartService,
			Parent: v.Parent,
			Payload: components.Resource{
				Parent: v.Parent.Name,
				Name:   fmt.Sprintf("%v-volume.service", service),
				Kind:   components.ResourceTypeService,
			},
		})
		if err != nil {
			return err
		}
	case ActionInstallVolumeFile:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.installVolumeFile(ctx, v.Parent, v.Payload); err != nil {
			return err
		}
	case ActionUpdateVolumeFile:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.installVolumeFile(ctx, v.Parent, v.Payload); err != nil {
			return err
		}
	case ActionRemoveVolumeFile:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
		if err := m.removeVolumeFile(ctx, v.Parent, v.Payload); err != nil {
			return err
		}
	case ActionSetupComponent:
		if err := m.CompRepo.RunSetup(v.Parent); err != nil {
			return err
		}
	case ActionStartService, ActionStopService, ActionRestartService, ActionEnableService, ActionDisableService:
		err := m.modifyService(ctx, v)
		if err != nil {
			return err
		}
	case ActionReloadUnits:
		err := m.modifyService(ctx, v)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action to execute: %v", v)
	}
	return nil
}

func (m *Materia) modifyService(ctx context.Context, command Action) error {
	if err := command.Validate(); err != nil {
		return err
	}
	var res components.Resource
	if command.Todo != ActionReloadUnits {
		res = command.Payload
		if err := res.Validate(); err != nil {
			return fmt.Errorf("invalid resource when modifying service: %w", err)
		}

		if res.Kind != components.ResourceTypeService {
			return errors.New("attempted to modify a non service resource")
		}
	}
	var cmd services.ServiceAction
	switch command.Todo {
	case ActionStartService:
		cmd = services.ServiceStart
		log.Debug("starting service", "unit", res.Name)
	case ActionStopService:
		log.Debug("stopping service", "unit", res.Name)
		cmd = services.ServiceStop
	case ActionRestartService:
		log.Debug("restarting service", "unit", res.Name)
		cmd = services.ServiceRestart
	case ActionReloadUnits:
		log.Debug("reloading units")
		cmd = services.ServiceReloadUnits
	case ActionEnableService:
		log.Debug("enabling service", "unit", res.Name)
		cmd = services.ServiceEnable
	case ActionDisableService:
		log.Debug("disabling service", "unit", res.Name)
		cmd = services.ServiceDisable
	case ActionReloadService:
		log.Debug("reloading service", "unit", res.Name)
		cmd = services.ServiceReloadService

	default:
		return errors.New("invalid service command")
	}
	return m.Services.Apply(ctx, res.Name, cmd)
}

func (m *Materia) installVolumeFile(ctx context.Context, parent *components.Component, res components.Resource) error {
	var vrConf *manifests.VolumeResourceConfig
	for _, vr := range parent.VolumeResources {
		if vr.Resource == res.Name {
			vrConf = &vr
			break
		}
	}
	if vrConf == nil {
		return fmt.Errorf("tried to install volume file for nonexistent volume resource: %v", res.Name)
	}
	vrConf.Volume = fmt.Sprintf("systemd-%v", vrConf.Volume)
	volumes, err := m.Containers.ListVolumes(ctx)
	if err != nil {
		return err
	}
	var volume *containers.Volume
	if !slices.ContainsFunc(volumes, func(v *containers.Volume) bool {
		if v.Name == vrConf.Volume {
			volume = v
			return true
		}
		return false
	}) {
		return fmt.Errorf("tried to install volume file into nonexistent volume: %v/%v", vrConf.Volume, res.Name)
	}
	inVolumeLoc := filepath.Join(volume.Mountpoint, vrConf.Path)
	data, err := os.ReadFile(res.Path)
	if err != nil {
		return err
	}
	mode := vrConf.Mode
	if mode == "" {
		mode = "0o755"
	}
	parsedMode, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return err
	}
	err = os.WriteFile(inVolumeLoc, bytes.NewBuffer(data).Bytes(), os.FileMode(parsedMode))
	if err != nil {
		return err
	}
	if vrConf.Owner != "" {
		uid, err := strconv.ParseInt(vrConf.Owner, 10, 32)
		if err != nil {
			return err
		}
		err = os.Chown(inVolumeLoc, int(uid), -1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Materia) removeVolumeFile(ctx context.Context, parent *components.Component, res components.Resource) error {
	var vrConf *manifests.VolumeResourceConfig
	for _, vr := range parent.VolumeResources {
		if vr.Resource == res.Name {
			vrConf = &vr
		}
	}
	if vrConf == nil {
		return fmt.Errorf("tried to remove volume file for nonexistent volume resource: /%v", res.Name)
	}
	vrConf.Volume = fmt.Sprintf("systemd-%v", vrConf.Volume)
	volumes, err := m.Containers.ListVolumes(ctx)
	if err != nil {
		return err
	}
	var volume *containers.Volume
	if !slices.ContainsFunc(volumes, func(v *containers.Volume) bool {
		if v.Name == vrConf.Volume {
			volume = v
			return true
		}
		return false
	}) {
		return fmt.Errorf("tried to remove volume file into nonexistent volume: %v/%v", vrConf.Volume, res.Name)
	}
	inVolumeLoc := filepath.Join(volume.Mountpoint, vrConf.Path)
	return os.Remove(inVolumeLoc)
}
