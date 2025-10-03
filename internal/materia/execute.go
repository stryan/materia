package materia

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
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
		attrs := make(map[string]any)
		if err := v.Validate(); err != nil {
			return steps, err
		}
		vaultAttrs := m.Attributes.Lookup(ctx, attributes.AttributesFilter{
			Hostname:  m.HostFacts.GetHostname(),
			Roles:     m.Roles,
			Component: v.Parent.Name,
		})
		maps.Copy(attrs, v.Parent.Defaults)
		maps.Copy(attrs, vaultAttrs)
		err := m.executeAction(ctx, v, attrs)
		if err != nil {
			return steps, err
		}

		if (v.Todo == ActionStart || v.Todo == ActionStop || v.Todo == ActionRestart || v.Todo == ActionEnable || v.Todo == ActionDisable || v.Todo == ActionReload) && v.Target.Kind == components.ResourceTypeService {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	activating := []string{}
	deactivating := []string{}
	for _, v := range serviceActions {
		serv, err := m.Services.Get(ctx, v.Target.Path)
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case ActionRestart, ActionStart:
			if serv.State == "activating" {
				activating = append(activating, v.Target.Path)
			} else if serv.State != "active" {
				log.Warn("service failed to start/restart", "service", serv.Name, "state", serv.State)
			}
		case ActionStop:
			if serv.State == "deactivating" {
				deactivating = append(deactivating, v.Target.Path)
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
	res := command.Target
	isUnits := command.Target.Kind == components.ResourceTypeHost
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

func (m *Materia) executeAction(ctx context.Context, v Action, attrs map[string]any) error {
	switch v.Target.Kind {
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
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeFile, components.ResourceTypeContainer, components.ResourceTypeVolume, components.ResourceTypePod, components.ResourceTypeNetwork, components.ResourceTypeKube, components.ResourceTypeManifest:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			resourceTemplate, err := m.SourceRepo.ReadResource(v.Target)
			if err != nil {
				return err
			}
			resourceData, err := m.executeResource(resourceTemplate, attrs)
			if err != nil {
				return err
			}
			if err := m.CompRepo.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Target); err != nil {
				return err
			}
		case ActionEnsure:
			if v.Target.Kind != components.ResourceTypeVolume {
				return fmt.Errorf("tried to ensure non volume resource: %v", v.Target)
			}
			service := strings.TrimSuffix(v.Target.Path, ".volume")
			err := m.modifyService(ctx, Action{
				Todo:   ActionStart,
				Parent: v.Parent,
				Target: components.Resource{
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
				return fmt.Errorf("cleanup is disabled: %v", v.Target)
			}
			switch v.Target.Kind {
			case components.ResourceTypeNetwork:
				err := m.Containers.RemoveNetwork(ctx, &containers.Network{Name: v.Target.HostObject})
				if err != nil {
					return err
				}
			case components.ResourceTypeVolume:
				if m.cleanupVolumes {
					err := m.Containers.RemoveVolume(ctx, &containers.Volume{Name: v.Target.HostObject})
					if err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("cleanup is not valid for this resource type: %v", v.Target)
			}
		case ActionDump:
			if v.Target.Kind != components.ResourceTypeVolume {
				return fmt.Errorf("tried to dump non volume resource: %v", v.Target)
			}
			err := m.Containers.DumpVolume(ctx, &containers.Volume{Name: v.Target.HostObject}, m.OutputDir, false)
			if err != nil {
				return fmt.Errorf("error dumping volume %v:%e", v.Target.Path, err)
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeScript:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			resourceTemplate, err := m.SourceRepo.ReadResource(v.Target)
			if err != nil {
				return err
			}
			resourceData, err := m.executeResource(resourceTemplate, attrs)
			if err != nil {
				return err
			}
			if err := m.CompRepo.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
			if err := m.ScriptRepo.Install(ctx, v.Target.Path, resourceData); err != nil {
				return err
			}

		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Target); err != nil {
				return err
			}
			if err := m.ScriptRepo.Remove(ctx, v.Target.Path); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeDirectory:
		switch v.Todo {
		case ActionInstall:
			if err := m.CompRepo.InstallResource(v.Target, nil); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Target); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
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
			resourceTemplate, err := m.SourceRepo.ReadResource(v.Target)
			if err != nil {
				return err
			}
			resourceData, err := m.executeResource(resourceTemplate, attrs)
			if err != nil {
				return err
			}
			if err := m.CompRepo.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
			if err := m.ServiceRepo.Install(ctx, v.Target.Path, resourceData); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Target); err != nil {
				return err
			}
			if err := m.ServiceRepo.Remove(ctx, v.Target.Path); err != nil {
				return err
			}
		case ActionStart, ActionStop, ActionEnable, ActionDisable, ActionReload, ActionRestart:
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeComponentScript:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			resourceTemplate, err := m.SourceRepo.ReadResource(v.Target)
			if err != nil {
				return err
			}
			resourceData, err := m.executeResource(resourceTemplate, attrs)
			if err != nil {
				return err
			}
			if err := m.CompRepo.InstallResource(v.Target, resourceData); err != nil {
				return err
			}

		case ActionRemove:
			if err := m.CompRepo.RemoveResource(v.Target); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypePodmanSecret:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			var secretVar any
			var ok bool
			if secretVar, ok = attrs[v.Target.Path]; !ok {
				return errors.New("can't install/update Podman Secret: no matching Materia secret")
			}
			if value, ok := secretVar.(string); !ok {
				return errors.New("can't install/update Podman Secret: materia secret isn't string")
			} else {
				if err := m.Containers.WriteSecret(ctx, v.Target.Path, value); err != nil {
					return err
				}
			}

		case ActionRemove:
			if err := m.Containers.RemoveSecret(ctx, v.Target.Path); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	default:
		panic(fmt.Sprintf("unexpected components.ResourceType: %v", v.Target.Kind))
	}

	return nil
}
