package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/services"
)

type ExecutorConfig struct {
	CleanupComponents bool   `toml:"cleanup_components"`
	MateriaDir        string `toml:"materia_dir"`
	QuadletDir        string `toml:"quadlet_dir"`
	ScriptsDir        string `toml:"scripts_dir"`
	ServiceDir        string `toml:"service_dir"`
}

func NewExecutorConfig(k *koanf.Koanf) (*ExecutorConfig, error) {
	ec := &ExecutorConfig{
		CleanupComponents: k.Bool("executor.cleanup_components"),
		MateriaDir:        k.String("executor.materia_dir"),
		QuadletDir:        k.String("executor.quadlet_dir"),
		ScriptsDir:        k.String("executor.scripts_dir"),
		ServiceDir:        k.String("executor.service_dir"),
	}

	return ec, nil
}

func (m *Materia) Execute(ctx context.Context, plan *Plan) (int, error) {
	if plan.Empty() {
		return -1, nil
	}
	defer func() {
		if !m.executorConfig.CleanupComponents {
			return
		}
		problems, err := m.ValidateComponents(ctx)
		if err != nil {
			log.Warnf("error cleaning up execution: %v", err)
		}
		for _, v := range problems {
			log.Infof("component %v failed to install, purging", v)
			err := m.Host.PurgeComponentByName(v)
			if err != nil {
				log.Warnf("error purging component: %v", err)
			}
		}
	}()
	serviceActions := []Action{}
	steps := 0
	// Execute resources
	for _, v := range plan.Steps() {
		err := m.executeAction(ctx, v)
		if err != nil {
			return steps, err
		}

		if v.Todo == ActionStart || v.Todo == ActionStop || v.Todo == ActionRestart || v.Todo == ActionEnable || v.Todo == ActionDisable || v.Todo == ActionReload && v.Target.Kind != components.ResourceTypeHost {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	servicesResultMap := make(map[string]string)
	for _, v := range serviceActions {
		serv, err := m.Host.Get(ctx, v.Target.Service())
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case ActionRestart, ActionStart, ActionReload:
			switch serv.State {
			case "activating", "reloading":
				servicesResultMap[serv.Name] = "active"
			case "failed":
				log.Warn("service failed to start/restart/reload", "service", serv.Name, "state", serv.State)
			default:
			}
		case ActionStop:
			if serv.State == "deactivating" {
				servicesResultMap[serv.Name] = "inactive"
			} else if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		case ActionEnable, ActionDisable:
		default:
			return steps, errors.New("unknown service action state")
		}
	}
	var servWG sync.WaitGroup
	for serv, state := range servicesResultMap {
		servWG.Add(1)
		go func(serv, state string) {
			defer servWG.Done()
			err := m.Host.WaitUntilState(ctx, serv, state, m.defaultTimeout)
			if err != nil {
				log.Warn(err)
			}
		}(serv, state)

	}
	servWG.Wait()
	return steps, nil
}

func (m *Materia) modifyService(ctx context.Context, command Action) error {
	if err := command.Validate(); err != nil {
		return err
	}
	res := command.Target
	if !res.IsQuadlet() && (res.Kind != components.ResourceTypeService && res.Kind != components.ResourceTypeHost) {
		return fmt.Errorf("tried to modify resource %v as a service", res)
	}
	isUnits := command.Target.Kind == components.ResourceTypeHost
	timeout := m.defaultTimeout
	if command.Metadata != nil && command.Metadata.ServiceTimeout != nil {
		timeout = *command.Metadata.ServiceTimeout
	}
	if err := res.Validate(); err != nil {
		return fmt.Errorf("invalid resource when modifying service: %w", err)
	}
	var cmd services.ServiceAction
	switch command.Todo {
	case ActionStart:
		cmd = services.ServiceStart
		log.Debug("starting service", "unit", res.Service())
	case ActionStop:
		log.Debug("stopping service", "unit", res.Service())
		cmd = services.ServiceStop
	case ActionRestart:
		log.Debug("restarting service", "unit", res.Service())
		cmd = services.ServiceRestart
	case ActionReload:
		if isUnits {
			log.Debug("reloading units")
			cmd = services.ServiceReloadUnits
		} else {
			log.Debug("reloading service", "unit", res.Service())
			cmd = services.ServiceReloadService
		}
	case ActionEnable:
		log.Debug("enabling service", "unit", res.Service())
		cmd = services.ServiceEnable
	case ActionDisable:
		log.Debug("disabling service", "unit", res.Service())
		cmd = services.ServiceDisable

	default:
		return errors.New("invalid service command")
	}
	return m.Host.Apply(ctx, res.Service(), cmd, timeout)
}

func (m *Materia) executeAction(ctx context.Context, v Action) error {
	switch v.Target.Kind {
	case components.ResourceTypeComponent:
		switch v.Todo {
		case ActionInstall:
			if err := m.Host.InstallComponent(v.Parent); err != nil {
				return err
			}
		case ActionUpdate:
			if err := m.Host.UpdateComponent(v.Parent); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveComponent(v.Parent); err != nil {
				return err
			}
		case ActionCleanup:
			if err := m.Host.RunCleanup(v.Parent); err != nil {
				return err
			}
		case ActionSetup:
			if err := m.Host.RunSetup(v.Parent); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeVolume:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := diffmatchpatch.New().DiffText2(diffs)
			if err := m.Host.InstallResource(v.Target, bytes.NewBufferString(resourceData)); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
				return err
			}
		case ActionEnsure:
			err := m.modifyService(ctx, Action{
				Todo:   ActionReload,
				Parent: rootComponent,
				Target: components.Resource{Kind: components.ResourceTypeHost},
			})
			if err != nil {
				return err
			}
			err = m.modifyService(ctx, Action{
				Todo:   ActionRestart,
				Parent: v.Parent,
				Target: components.Resource{
					Parent: v.Parent.Name,
					Path:   v.Target.Service(),
					Kind:   components.ResourceTypeService,
				},
			})
			if err != nil {
				return err
			}
		case ActionCleanup:
			containersWithVolume, err := m.Host.ListContainers(ctx, containers.ContainerListFilter{
				Volume: v.Target.HostObject,
				All:    true,
			})
			if err != nil {
				return fmt.Errorf("can't cleanup volume %v: %w", v.Target, err)
			}
			if len(containersWithVolume) > 0 {
				log.Warnf("skipping cleaning up volume %v since it's still in use", v.Target.Path)
			} else {
				err = m.Host.RemoveVolume(ctx, &containers.Volume{Name: v.Target.HostObject})
				if err != nil {
					return err
				}
			}

		case ActionDump:
			err := m.Host.DumpVolume(ctx, &containers.Volume{Name: v.Target.HostObject}, m.OutputDir, false)
			if err != nil {
				return fmt.Errorf("error dumping volume %v:%w", v.Target.Path, err)
			}
		case ActionImport:
			err := m.Host.ImportVolume(ctx, &containers.Volume{Name: v.Target.HostObject, Driver: "local"}, filepath.Join(m.OutputDir, fmt.Sprintf("%v.tar", v.Target.HostObject)))
			if err != nil {
				return fmt.Errorf("error importing volume %v: %w", v.Target.HostObject, err)
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)

		}
	case components.ResourceTypeContainer, components.ResourceTypePod, components.ResourceTypeNetwork, components.ResourceTypeKube, components.ResourceTypeBuild, components.ResourceTypeImage:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := diffmatchpatch.New().DiffText2(diffs)
			if err := m.Host.InstallResource(v.Target, bytes.NewBufferString(resourceData)); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
				return err
			}
		case ActionCleanup:
			switch v.Target.Kind {
			case components.ResourceTypeNetwork:
				network, err := m.Host.GetNetwork(ctx, v.Target.HostObject)
				if err != nil {
					return fmt.Errorf("can't cleanup network %v: %w", v.Target, err)
				}
				if len(network.Containers) < 1 {
					err := m.Host.RemoveNetwork(ctx, &containers.Network{Name: v.Target.HostObject})
					if err != nil {
						return err
					}
				} else {
					log.Warnf("skipping cleaning up network %v since its still in use", v.Target.Path)
				}
			case components.ResourceTypeBuild, components.ResourceTypeImage:
				containerWithImage, err := m.Host.ListContainers(ctx, containers.ContainerListFilter{
					Image: v.Target.HostObject,
					All:   true,
				})
				if err != nil {
					return fmt.Errorf("can't cleanup image/build %v: %w", v.Target.HostObject, err)
				}
				if len(containerWithImage) > 0 {
					log.Warnf("skipping cleaning up image %v since it's still in use", v.Target.Path)
				} else {
					err = m.Host.RemoveImage(ctx, v.Target.HostObject)
					if err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("cleanup is not valid for this resource type: %v", v.Target)
			}
		case ActionEnsure:
			err := m.modifyService(ctx, Action{
				Todo:   ActionReload,
				Parent: rootComponent,
				Target: components.Resource{Kind: components.ResourceTypeHost},
			})
			if err != nil {
				return err
			}
			err = m.modifyService(ctx, Action{
				Todo:   ActionRestart,
				Parent: v.Parent,
				Target: components.Resource{
					Parent: v.Parent.Name,
					Path:   v.Target.Service(),
					Kind:   components.ResourceTypeService,
				},
			})
			if err != nil {
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
	case components.ResourceTypeFile, components.ResourceTypeManifest:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := diffmatchpatch.New().DiffText2(diffs)
			if err := m.Host.InstallResource(v.Target, bytes.NewBufferString(resourceData)); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeScript:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := bytes.NewBufferString(diffmatchpatch.New().DiffText2(diffs))
			if err := m.Host.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
			if err := m.Host.InstallScript(ctx, v.Target.Path, resourceData); err != nil {
				return err
			}

		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
				return err
			}
			if err := m.Host.RemoveScript(ctx, v.Target.Path); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeDirectory:
		switch v.Todo {
		case ActionInstall:
			if err := m.Host.InstallResource(v.Target, nil); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
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
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := bytes.NewBufferString(diffmatchpatch.New().DiffText2(diffs))
			if err := m.Host.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
			if err := m.Host.InstallUnit(ctx, v.Target.Path, resourceData); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
				return err
			}
			if err := m.Host.RemoveUnit(ctx, v.Target.Path); err != nil {
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
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := bytes.NewBufferString(diffmatchpatch.New().DiffText2(diffs))
			if err := m.Host.InstallResource(v.Target, resourceData); err != nil {
				return err
			}

		case ActionRemove:
			if err := m.Host.RemoveResource(v.Target); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypePodmanSecret:
		switch v.Todo {
		case ActionInstall, ActionUpdate:
			if err := m.Host.WriteSecret(ctx, v.Target.Path, v.Target.Content); err != nil {
				return err
			}
		case ActionRemove:
			if err := m.Host.RemoveSecret(ctx, v.Target.Path); err != nil {
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
