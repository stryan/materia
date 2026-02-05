package executor

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
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/serviceman"
)

type Host interface {
	serviceman.ServiceManager
	containers.ContainerManager
	components.ComponentWriter
	InstallScript(context.Context, string, *bytes.Buffer) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, *bytes.Buffer) error
	RemoveUnit(context.Context, string) error
}

type ExecutorConfig struct {
	CleanupComponents bool   `toml:"cleanup_components"`
	MateriaDir        string `toml:"materia_dir"`
	QuadletDir        string `toml:"quadlet_dir"`
	ScriptsDir        string `toml:"scripts_dir"`
	ServiceDir        string `toml:"service_dir"`
	OutputDir         string `toml:"output_dir"`
}

type Executor struct {
	ExecutorConfig
	host           Host
	defaultTimeout int
}

func (e *ExecutorConfig) String() string {
	return fmt.Sprintf("Cleanup Components: %v\nMateria Data Dir: %v\nQuadlets Dir: %v\nScripts Dir: %v\nService Dir: %v\n", e.CleanupComponents, e.MateriaDir, e.QuadletDir, e.ScriptsDir, e.ServiceDir)
}

func NewExecutorConfig(k *koanf.Koanf) (*ExecutorConfig, error) {
	ec := &ExecutorConfig{
		CleanupComponents: k.Bool("executor.cleanup_components"),
		MateriaDir:        k.String("executor.materia_dir"),
		QuadletDir:        k.String("executor.quadlet_dir"),
		ScriptsDir:        k.String("executor.scripts_dir"),
		ServiceDir:        k.String("executor.service_dir"),
		OutputDir:         k.String("executor.output_dir"),
	}

	return ec, nil
}

func NewExecutor(conf ExecutorConfig, host Host, timeout int) *Executor {
	return &Executor{
		conf,
		host,
		timeout,
	}
}

func getServiceType(a actions.Action) (services.ServiceAction, error) {
	switch a.Todo {
	case actions.ActionDisable:
		return services.ServiceDisable, nil
	case actions.ActionEnable:
		return services.ServiceEnable, nil
	case actions.ActionReload:
		if a.Target.Kind == components.ResourceTypeHost {
			return services.ServiceReloadUnits, nil
		}
		return services.ServiceReloadService, nil
	case actions.ActionRestart:
		return services.ServiceRestart, nil
	case actions.ActionStart:
		return services.ServiceStart, nil
	case actions.ActionStop:
		return services.ServiceStop, nil
	default:
		panic(fmt.Sprintf("unexpected actions.ActionType: %#v", a))
	}
}

func modifyService(ctx context.Context, sm serviceman.ServiceManager, command actions.Action, timeout int) error {
	if err := command.Validate(); err != nil {
		return err
	}
	res := command.Target
	if !res.IsQuadlet() && (res.Kind != components.ResourceTypeService && res.Kind != components.ResourceTypeHost) {
		return fmt.Errorf("tried to modify resource %v as a service", res)
	}
	if command.Metadata != nil && command.Metadata.ServiceTimeout != nil {
		timeout = *command.Metadata.ServiceTimeout
	}
	if err := res.Validate(); err != nil {
		return fmt.Errorf("invalid resource when modifying service: %w", err)
	}
	cmd, err := getServiceType(command)
	if err != nil {
		return err
	}
	log.Debug("%v service", "unit", cmd, res.Service())

	return sm.Apply(ctx, res.Service(), cmd, timeout)
}

func (e *Executor) executeAction(ctx context.Context, v actions.Action) error {
	rootComponent := components.NewComponent("root")
	switch v.Target.Kind {
	case components.ResourceTypeComponent:
		switch v.Todo {
		case actions.ActionInstall:
			if err := e.host.InstallComponent(v.Parent); err != nil {
				return err
			}
		case actions.ActionUpdate:
			if err := e.host.UpdateComponent(v.Parent); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveComponent(v.Parent); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeVolume:
		switch v.Todo {
		case actions.ActionInstall, actions.ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := diffmatchpatch.New().DiffText2(diffs)
			if err := e.host.InstallResource(v.Target, bytes.NewBufferString(resourceData)); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveResource(v.Target); err != nil {
				return err
			}
		case actions.ActionEnsure:
			err := modifyService(ctx, e.host, actions.Action{
				Todo:   actions.ActionReload,
				Parent: rootComponent,
				Target: components.Resource{Kind: components.ResourceTypeHost},
			}, e.defaultTimeout)
			if err != nil {
				return err
			}
			err = modifyService(ctx, e.host, actions.Action{
				Todo:   actions.ActionRestart,
				Parent: v.Parent,
				Target: components.Resource{
					Parent: v.Parent.Name,
					Path:   v.Target.Service(),
					Kind:   components.ResourceTypeService,
				},
			}, e.defaultTimeout)
			if err != nil {
				return err
			}
		case actions.ActionCleanup:
			containersWithVolume, err := e.host.ListContainers(ctx, containers.ContainerListFilter{
				Volume: v.Target.HostObject,
				All:    true,
			})
			if err != nil {
				return fmt.Errorf("can't cleanup volume %v: %w", v.Target, err)
			}
			if len(containersWithVolume) > 0 {
				log.Warnf("skipping cleaning up volume %v since it's still in use", v.Target.Path)
			} else {
				err = e.host.RemoveVolume(ctx, &containers.Volume{Name: v.Target.HostObject})
				if err != nil {
					return err
				}
			}

		case actions.ActionDump:
			err := e.host.DumpVolume(ctx, &containers.Volume{Name: v.Target.HostObject}, e.OutputDir, false)
			if err != nil {
				return fmt.Errorf("error dumping volume %v:%w", v.Target.Path, err)
			}
		case actions.ActionImport:
			err := e.host.ImportVolume(ctx, &containers.Volume{Name: v.Target.HostObject, Driver: "local"}, filepath.Join(e.OutputDir, fmt.Sprintf("%v.tar", v.Target.HostObject)))
			if err != nil {
				return fmt.Errorf("error importing volume %v: %w", v.Target.HostObject, err)
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)

		}
	case components.ResourceTypeContainer, components.ResourceTypePod, components.ResourceTypeNetwork, components.ResourceTypeKube, components.ResourceTypeBuild, components.ResourceTypeImage:
		switch v.Todo {
		case actions.ActionInstall, actions.ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := diffmatchpatch.New().DiffText2(diffs)
			if err := e.host.InstallResource(v.Target, bytes.NewBufferString(resourceData)); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveResource(v.Target); err != nil {
				return err
			}
		case actions.ActionCleanup:
			switch v.Target.Kind {
			case components.ResourceTypeNetwork:
				network, err := e.host.GetNetwork(ctx, v.Target.HostObject)
				if err != nil {
					return fmt.Errorf("can't cleanup network %v: %w", v.Target, err)
				}
				if len(network.Containers) < 1 {
					err := e.host.RemoveNetwork(ctx, &containers.Network{Name: v.Target.HostObject})
					if err != nil {
						return err
					}
				} else {
					log.Warnf("skipping cleaning up network %v since its still in use", v.Target.Path)
				}
			case components.ResourceTypeBuild, components.ResourceTypeImage:
				containerWithImage, err := e.host.ListContainers(ctx, containers.ContainerListFilter{
					Image: v.Target.HostObject,
					All:   true,
				})
				if err != nil {
					return fmt.Errorf("can't cleanup image/build %v: %w", v.Target.HostObject, err)
				}
				if len(containerWithImage) > 0 {
					log.Warnf("skipping cleaning up image %v since it's still in use", v.Target.Path)
				} else {
					err = e.host.RemoveImage(ctx, v.Target.HostObject)
					if err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("cleanup is not valid for this resource type: %v", v.Target)
			}
		case actions.ActionEnsure:
			err := modifyService(ctx, e.host, actions.Action{
				Todo:   actions.ActionReload,
				Parent: rootComponent,
				Target: components.Resource{Kind: components.ResourceTypeHost},
			}, e.defaultTimeout)
			if err != nil {
				return err
			}
			err = modifyService(ctx, e.host, actions.Action{
				Todo:   actions.ActionRestart,
				Parent: v.Parent,
				Target: components.Resource{
					Parent: v.Parent.Name,
					Path:   v.Target.Service(),
					Kind:   components.ResourceTypeService,
				},
			}, e.defaultTimeout)
			if err != nil {
				return err
			}
		case actions.ActionStart, actions.ActionStop, actions.ActionEnable, actions.ActionDisable, actions.ActionReload, actions.ActionRestart:
			err := modifyService(ctx, e.host, v, e.defaultTimeout)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeFile, components.ResourceTypeManifest:
		switch v.Todo {
		case actions.ActionInstall, actions.ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := diffmatchpatch.New().DiffText2(diffs)
			if err := e.host.InstallResource(v.Target, bytes.NewBufferString(resourceData)); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveResource(v.Target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeScript:
		switch v.Todo {
		case actions.ActionInstall, actions.ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := bytes.NewBufferString(diffmatchpatch.New().DiffText2(diffs))
			if err := e.host.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
			if err := e.host.InstallScript(ctx, v.Target.Path, resourceData); err != nil {
				return err
			}

		case actions.ActionRemove:
			if err := e.host.RemoveResource(v.Target); err != nil {
				return err
			}
			if err := e.host.RemoveScript(ctx, v.Target.Path); err != nil {
				return err
			}
		case actions.ActionSetup:
			scriptPath := filepath.Join(e.ScriptsDir, v.Target.Path)
			setupName := fmt.Sprintf("%v-materia-setup.service", v.Parent.Name)
			cleanupName := fmt.Sprintf("%v-materia-cleanup.service", v.Parent.Name)
			if err := e.host.RunOneshotCommand(ctx, e.defaultTimeout, setupName, []string{scriptPath}); err != nil {
				return err
			}
			// we succesfully setup, remove any cleanup script instances
			if err := e.host.Apply(ctx, cleanupName, services.ServiceStop, e.defaultTimeout); err != nil {
				log.Warnf("couldn't remove old cleanup script instance for %v: %v", v.Parent.Name, err)
			}
		case actions.ActionCleanup:
			scriptPath := filepath.Join(e.ScriptsDir, v.Target.Path)
			setupName := fmt.Sprintf("%v-materia-setup.service", v.Parent.Name)
			cleanupName := fmt.Sprintf("%v-materia-cleanup.service", v.Parent.Name)
			if err := e.host.RunOneshotCommand(ctx, e.defaultTimeout, cleanupName, []string{scriptPath}); err != nil {
				return err
			}
			// we succesfully setup, remove any setup script instances
			if err := e.host.Apply(ctx, setupName, services.ServiceStop, e.defaultTimeout); err != nil {
				log.Warnf("couldn't remove old cleanup script instance for %v: %v", v.Parent.Name, err)
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeDirectory:
		switch v.Todo {
		case actions.ActionInstall:
			if err := e.host.InstallResource(v.Target, nil); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveResource(v.Target); err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypeHost:
		if v.Todo != actions.ActionReload {
			return fmt.Errorf(" invalid action type %v for host resource", v.Todo)
		}
		err := modifyService(ctx, e.host, v, e.defaultTimeout)
		if err != nil {
			return err
		}
	case components.ResourceTypeService:
		switch v.Todo {
		case actions.ActionInstall, actions.ActionUpdate:
			diffs, err := v.GetContentAsDiffs()
			if err != nil {
				return err
			}
			resourceData := bytes.NewBufferString(diffmatchpatch.New().DiffText2(diffs))
			if err := e.host.InstallResource(v.Target, resourceData); err != nil {
				return err
			}
			if err := e.host.InstallUnit(ctx, v.Target.Path, resourceData); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveResource(v.Target); err != nil {
				return err
			}
			if err := e.host.RemoveUnit(ctx, v.Target.Path); err != nil {
				return err
			}
		case actions.ActionStart, actions.ActionStop, actions.ActionEnable, actions.ActionDisable, actions.ActionReload, actions.ActionRestart:
			err := modifyService(ctx, e.host, v, e.defaultTimeout)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid action type %v for resource %v", v.Todo, v.Target.Kind)
		}
	case components.ResourceTypePodmanSecret:
		switch v.Todo {
		case actions.ActionInstall, actions.ActionUpdate:
			if err := e.host.WriteSecret(ctx, v.Target.Path, v.Target.Content); err != nil {
				return err
			}
		case actions.ActionRemove:
			if err := e.host.RemoveSecret(ctx, v.Target.Path); err != nil {
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

func (e *Executor) Execute(ctx context.Context, plan *plan.Plan) (int, error) {
	if plan.Empty() {
		return -1, nil
	}
	serviceActions := []actions.Action{}
	steps := 0
	// Execute resources
	for _, v := range plan.Steps() {
		err := e.executeAction(ctx, v)
		if err != nil {
			return steps, err
		}

		if v.Todo == actions.ActionStart || v.Todo == actions.ActionStop || v.Todo == actions.ActionRestart || v.Todo == actions.ActionEnable || v.Todo == actions.ActionDisable || v.Todo == actions.ActionReload && v.Target.Kind != components.ResourceTypeHost {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	servicesResultMap := make(map[string]string)
	for _, v := range serviceActions {
		serv, err := e.host.Get(ctx, v.Target.Service())
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case actions.ActionRestart, actions.ActionStart, actions.ActionReload:
			switch serv.State {
			case "activating", "reloading":
				servicesResultMap[serv.Name] = "active"
			case "failed":
				log.Warn("service failed to start/restart/reload", "service", serv.Name, "state", serv.State)
			default:
			}
		case actions.ActionStop:
			if serv.State == "deactivating" {
				servicesResultMap[serv.Name] = "inactive"
			} else if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		case actions.ActionEnable, actions.ActionDisable:
		default:
			return steps, errors.New("unknown service action state")
		}
	}
	var servWG sync.WaitGroup
	for serv, state := range servicesResultMap {
		servWG.Add(1)
		go func(serv, state string) {
			defer servWG.Done()
			err := e.host.WaitUntilState(ctx, serv, state, e.defaultTimeout)
			if err != nil {
				log.Warn(err)
			}
		}(serv, state)

	}
	servWG.Wait()
	return steps, nil
}
