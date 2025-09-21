package materia

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
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

		if (v.Todo == ActionStart || v.Todo == ActionStop || v.Todo == ActionRestart || v.Todo == ActionEnable || v.Todo == ActionDisable || v.Todo == ActionReload) && v.Payload.Kind == components.ResourceTypeService {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	activating := []string{}
	deactivating := []string{}
	for _, v := range serviceActions {
		serv, err := m.Services.Get(ctx, v.Payload.Path)
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case ActionRestart, ActionStart:
			if serv.State == "activating" {
				activating = append(activating, v.Payload.Path)
			} else if serv.State != "active" {
				log.Warn("service failed to start/restart", "service", serv.Name, "state", serv.State)
			}
		case ActionStop:
			if serv.State == "deactivating" {
				deactivating = append(deactivating, v.Payload.Path)
			} else if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		case ActionEnable, ActionDisable:
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

func (m *Materia) modifyService(ctx context.Context, command Action) error {
	if err := command.Validate(); err != nil {
		return err
	}
	res := command.Payload
	isUnits := command.Payload.Kind == components.ResourceTypeHost
	if !isUnits {
		if err := res.Validate(); err != nil {
			return fmt.Errorf("invalid resource when modifying service: %w", err)
		}

		if res.Kind != components.ResourceTypeService {
			return errors.New("attempted to modify a non service resource")
		}
	}
	var cmd services.ServiceAction
	switch command.Todo {
	case ActionStart:
		cmd = services.ServiceStart
		log.Debug("starting service", "unit", res)
	case ActionStop:
		log.Debug("stopping service", "unit", res)
		cmd = services.ServiceStop
	case ActionRestart:
		log.Debug("restarting service", "unit", res)
		cmd = services.ServiceRestart
	case ActionReload:
		if isUnits {
			log.Debug("reloading units")
			cmd = services.ServiceReloadUnits
		} else {
			log.Debug("reloading service", "unit", res)
			cmd = services.ServiceReloadService
		}
	case ActionEnable:
		log.Debug("enabling service", "unit", res)
		cmd = services.ServiceEnable
	case ActionDisable:
		log.Debug("disabling service", "unit", res)
		cmd = services.ServiceDisable

	default:
		return errors.New("invalid service command")
	}
	return m.Services.Apply(ctx, res.Path, cmd)
}

func (m *Materia) executeAction(ctx context.Context, v Action, vars map[string]any) error {
	switch v.Payload.Kind {
	case components.ResourceTypeComponent:
		switch v.Todo {
		case ActionInstall:
			if err := m.CompRepo.InstallComponent(v.Parent); err != nil {
				return err
			}
		case ActionUpdate:
			if err := m.CompRepo.UpdateComponent(v.Parent); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.CompRepo.RemoveComponent(v.Parent); err != nil {
				return err
			}
		case ActionCleanup:
			if err := m.CompRepo.RunCleanup(v.Parent); err != nil {
				return err
			}
		case ActionSetup:
			if err := m.CompRepo.RunSetup(v.Parent); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	case components.ResourceTypeFile, components.ResourceTypeContainer, components.ResourceTypeVolume, components.ResourceTypePod, components.ResourceTypeNetwork, components.ResourceTypeKube, components.ResourceTypeManifest:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
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
		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
				return err
			}
		case ActionEnsure:
			if v.Payload.Kind != components.ResourceTypeVolume {
				return fmt.Errorf("tried to ensure non volume resource: %v", v.Payload)
			}
			service := strings.TrimSuffix(v.Payload.Path, ".volume")
			err := m.modifyService(ctx, Action{
				Todo:   ActionStart,
				Parent: v.Parent,
				Payload: components.Resource{
					Parent: v.Parent.Name,
					Path:   fmt.Sprintf("%v-volume.service", service),
					Kind:   components.ResourceTypeService,
				},
			})
			if err != nil {
				return err
			}
		case ActionCleanup:
			if !m.cleanup {
				return fmt.Errorf("cleanup is disabled: %v", v.Payload)
			}
			switch v.Payload.Kind {
			case components.ResourceTypeNetwork:
				err := m.Containers.RemoveNetwork(ctx, &containers.Network{Name: v.Payload.HostObject})
				if err != nil {
					return err
				}
			case components.ResourceTypeVolume:
				if m.cleanupVolumes {
					err := m.Containers.RemoveVolume(ctx, &containers.Volume{Name: v.Payload.HostObject})
					if err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("cleanup is not valid for this resource type: %v", v.Payload)
			}
		case ActionDump:
			if v.Payload.Kind != components.ResourceTypeVolume {
				return fmt.Errorf("tried to dump non volume resource: %v", v.Payload)
			}
			err := m.Containers.DumpVolume(ctx, &containers.Volume{Name: v.Payload.HostObject}, m.OutputDir, false)
			if err != nil {
				return fmt.Errorf("error dumping volume %v:%e", v.Payload.Path, err)
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	case components.ResourceTypeScript:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
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
			if err := m.ScriptRepo.Install(ctx, v.Payload.Path, resourceData); err != nil {
				return err
			}

		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
				return err
			}
			if err := m.ScriptRepo.Remove(ctx, v.Payload.Path); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	case components.ResourceTypeDirectory:
		switch v.Todo {
		case ActionInstall:
			if err := m.CompRepo.InstallResource(v.Payload, nil); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	case components.ResourceTypeHost:
		if v.Todo != ActionReload {
			return fmt.Errorf(" invalid action type %v for host resource", v.Todo)
		}
		err := m.modifyService(ctx, v)
		if err != nil {
			return err
		}
	case components.ResourceTypeService:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
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
			if err := m.ServiceRepo.Install(ctx, v.Payload.Path, resourceData); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
				return err
			}
			if err := m.ServiceRepo.Remove(ctx, v.Payload.Path); err != nil {
				return err
			}
		case ActionStart, ActionStop, ActionEnable, ActionDisable, ActionReload, ActionRestart:
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	case components.ResourceTypeComponentScript:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
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

		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	case components.ResourceTypePodmanSecret:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			var secretVar any
			var ok bool
			if secretVar, ok = vars[v.Payload.Path]; !ok {
				return errors.New("can't install/update Podman Secret: no matching Materia secret")
			}
			if value, ok := secretVar.(string); !ok {
				return errors.New("can't install/update Podman Secret: materia secret isn't string")
			} else {
				if err := m.Containers.WriteSecret(ctx, v.Payload.Path, value); err != nil {
					return err
				}
			}

		case ActionRemove:
			if err := m.Containers.RemoveSecret(ctx, v.Payload.Path); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Payload.Kind)
		}
	default:
		panic(fmt.Sprintf("unexpected components.ResourceType: %v", v.Payload.Kind))
	}

	return nil
}
