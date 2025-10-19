// Package materia contains the primary materia plan-execute functions. You probably don't want to be calling it
package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/internal/source/file"
	"primamateria.systems/materia/internal/source/git"
)

type MacroMap func(map[string]any) template.FuncMap

// TODO ugly hack, remove
var rootComponent = &components.Component{Name: "root"}

type Materia struct {
	HostFacts           FactsProvider
	Manifest            *manifests.MateriaManifest
	Services            Services
	Containers          ContainerManager
	Attributes          AttributesEngine
	source              Source
	CompRepo            ComponentRepository
	ScriptRepo          Repository
	ServiceRepo         Repository
	SourceRepo          ComponentRepository
	rootComponent       *components.Component
	AssignedComponents  []string
	InstalledComponents []string
	Roles               []string
	macros              MacroMap
	snippets            map[string]*Snippet
	OutputDir           string
	onlyResources       bool
	debug               bool
	diffs               bool
	cleanup             bool
	cleanupVolumes      bool
	backupVolumes       bool
	migrateVolumes      bool
	remoteDir           string
}

func NewMateria(ctx context.Context, c *MateriaConfig, source Source, facts FactsProvider, attributes AttributesEngine, sm Services, cm ContainerManager, scriptRepo, serviceRepo Repository, sourceRepo ComponentRepository, hostRepo ComponentRepository) (*Materia, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var err error
	err = source.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("error syncing source: %w", err)
	}
	snips := make(map[string]*Snippet)
	defaultSnippets := loadDefaultSnippets()
	for _, v := range defaultSnippets {
		snips[v.Name] = v
	}
	// load manifest

	manifestLocation := filepath.Join(c.SourceDir, manifests.MateriaManifestFile)
	man, err := manifests.LoadMateriaManifest(manifestLocation)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, fmt.Errorf("invalid materia manifest: %w", err)
	}

	m := &Materia{
		Services:       sm,
		Containers:     cm,
		HostFacts:      facts,
		Manifest:       man,
		source:         source,
		debug:          c.Debug,
		diffs:          c.Diffs,
		cleanup:        c.Cleanup,
		onlyResources:  c.OnlyResources,
		Attributes:     attributes,
		CompRepo:       hostRepo,
		ScriptRepo:     scriptRepo,
		ServiceRepo:    serviceRepo,
		SourceRepo:     sourceRepo,
		OutputDir:      c.OutputDir,
		snippets:       snips,
		rootComponent:  rootComponent,
		cleanupVolumes: c.CleanupVolumes,
		backupVolumes:  c.BackupVolumes,
		migrateVolumes: c.MigrateVolumes,
	}
	m.macros = func(vars map[string]any) template.FuncMap {
		return template.FuncMap{
			"m_deps": func(arg string) (string, error) {
				switch arg {
				case "after":
					if res, ok := vars["After"]; ok {
						return res.(string), nil
					} else {
						return "local-fs.target network.target", nil
					}
				case "wants":
					if res, ok := vars["Wants"]; ok {
						return res.(string), nil
					} else {
						return "local-fs.target network.target", nil
					}
				case "requires":
					if res, ok := vars["Requires"]; ok {
						return res.(string), nil
					} else {
						return "local-fs.target network.target", nil
					}
				default:
					return "", errors.New("err bad default")
				}
			},
			"m_dataDir": func(arg string) (string, error) {
				return filepath.Join(filepath.Join(c.MateriaDir, "materia", "components"), arg), nil
			},
			"m_facts": func(arg string) (any, error) {
				return m.HostFacts.Lookup(arg)
			},
			"m_default": func(arg string, def string) string {
				val, ok := vars[arg]
				if ok {
					return val.(string)
				}
				return def
			},
			"exists": func(arg string) bool {
				_, ok := vars[arg]
				return ok
			},
			"secretEnv": func(args ...string) string {
				if len(args) == 0 {
					return ""
				}
				if len(args) == 1 {
					return fmt.Sprintf("Secret=%v,type=env,target=%v", m.Containers.SecretName(args[0]), args[0])
				}
				return fmt.Sprintf("Secret=%v,type=env,target=%v", m.Containers.SecretName(args[0]), args[1])
			},
			"secretMount": func(args ...string) string {
				if len(args) == 0 {
					return ""
				}
				if len(args) == 1 {
					return fmt.Sprintf("Secret=%v,type=mount,target=%v", m.Containers.SecretName(args[0]), args[0])
				}
				return fmt.Sprintf("Secret=%v,type=env,%s", m.Containers.SecretName(args[0]), args[1])
			},
			"snippet": func(name string, args ...string) (string, error) {
				s, ok := m.snippets[name]
				if !ok {
					return "", errors.New("snippet not found")
				}
				snipVars := make(map[string]string, len(s.Parameters))
				for k, v := range s.Parameters {
					snipVars[v] = args[k]
				}

				result := bytes.NewBuffer([]byte{})
				err := s.Body.Execute(result, snipVars)
				return result.String(), err
			},
		}
	}
	m.InstalledComponents, err = m.CompRepo.ListComponentNames()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed components: %w", err)
	}

	slices.Sort(m.InstalledComponents)
	if man == nil {
		// bail out early since the rest of this needs manifests
		return m, nil
	}

	for _, v := range m.Manifest.Snippets {
		s, err := configToSnippet(v)
		if err != nil {
			return nil, err
		}
		m.snippets[s.Name] = s
	}
	host, ok := man.Hosts["all"]
	if ok {
		m.AssignedComponents = append(m.AssignedComponents, host.Components...)
	}
	host, ok = man.Hosts[facts.GetHostname()]
	if ok {
		m.AssignedComponents = append(m.AssignedComponents, host.Components...)
	}
	m.Roles, err = getRolesFromManifest(man, facts.GetHostname())
	if err != nil {
		return nil, fmt.Errorf("unable to load roles form manifest: %w", err)
	}
	for _, v := range m.Roles {
		if len(man.Roles[v].Components) != 0 {
			m.AssignedComponents = append(m.AssignedComponents, man.Roles[v].Components...)
		}
	}
	slices.Sort(m.AssignedComponents)
	return m, nil
}

func getRolesFromManifest(man *manifests.MateriaManifest, hostname string) ([]string, error) {
	var roles []string
	if man.RoleCommand != "" {
		roleStruct := struct{ Roles []string }{}
		cmd := exec.Command(man.RoleCommand)
		res, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		err = toml.Unmarshal(res, &roleStruct)
		if err != nil {
			return nil, err
		}
		roles = append(roles, roleStruct.Roles...)
	} else if host, ok := man.Hosts[hostname]; ok {
		if len(host.Roles) != 0 {
			roles = append(roles, host.Roles...)
		}
	}
	return roles, nil
}

func (m *Materia) SyncRemote(ctx context.Context) error {
	if len(m.Manifest.Remotes) > 0 {
		for name, r := range m.Manifest.Remotes {
			parsedPath := strings.Split(r.URL, "://")
			var remoteSource Source
			var err error
			switch parsedPath[0] {
			case "git":
				localpath := filepath.Join(m.remoteDir, "components", name)
				remoteSource, err = git.NewGitSource(&git.Config{
					Branch:           r.Version,
					PrivateKey:       "",
					Username:         "",
					Password:         "",
					KnownHosts:       "",
					Insecure:         false,
					LocalRepository:  localpath,
					RemoteRepository: parsedPath[1],
				})
				if err != nil {
					return fmt.Errorf("invalid git source: %w", err)
				}
			case "file":
				localpath := filepath.Join(m.remoteDir, "components", name)
				remoteSource, err = file.NewFileSource(&file.Config{
					SourcePath:  parsedPath[1],
					Destination: localpath,
				})
				if err != nil {
					return fmt.Errorf("invalid file source: %w", err)
				}
			default:
				return fmt.Errorf("invalid source: %v", parsedPath[0])
			}
			if err := remoteSource.Sync(ctx); err != nil {
				return err
			}

		}
	}
	// remove old remote components to keep things tidy
	// TODO maybe the ugliness of doing this here means its worth having a seperate engine for remote components
	entries, err := os.ReadDir(filepath.Join(m.remoteDir, "components"))
	if err != nil {
		return err
	}
	for _, v := range entries {
		if v.IsDir() {
			if _, ok := m.Manifest.Remotes[v.Name()]; !ok {
				log.Debugf("Removing old remote component %v", v.Name())
				err := os.RemoveAll(filepath.Join(m.remoteDir, "components", v.Name()))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Materia) Close() {
	m.Services.Close()
	m.Containers.Close()
}

func (m *Materia) Clean(ctx context.Context) error {
	err := m.CompRepo.Clean()
	if err != nil {
		return err
	}
	err = m.SourceRepo.Clean()
	if err != nil {
		return err
	}
	return os.RemoveAll(m.OutputDir)
}

func (m *Materia) CleanComponent(ctx context.Context, name string) error {
	isInstalled := slices.Contains(m.InstalledComponents, name)
	if !isInstalled {
		return errors.New("component not installed")
	}
	comp, err := m.CompRepo.GetComponent(name)
	if err != nil {
		return err
	}
	m.InstalledComponents = []string{comp.Name}
	m.AssignedComponents = []string{}

	removalPlan, err := m.Plan(ctx)
	if err != nil {
		return err
	}
	_, err = m.Execute(ctx, removalPlan)
	return err
}

func (m *Materia) PlanComponent(ctx context.Context, name string, roles []string) (*Plan, error) {
	if roles != nil {
		m.Roles = roles
	}
	if name != "" {
		m.AssignedComponents = []string{name}
	}
	m.Services = &services.PlannedServiceManager{}
	m.InstalledComponents = []string{}
	return m.Plan(ctx)
}

func (m *Materia) ValidateComponents(ctx context.Context) ([]string, error) {
	var invalidComps []string
	dcomps, err := m.CompRepo.ListComponentNames()
	if err != nil {
		return invalidComps, fmt.Errorf("can't get components from prefix: %w", err)
	}
	for _, name := range dcomps {
		_, err = m.CompRepo.GetComponent(name)
		if err != nil {
			log.Warn("component unable to be loaded", "component", name)
			invalidComps = append(invalidComps, name)
		}
	}

	return invalidComps, nil
}

func (m *Materia) PurgeComponenet(ctx context.Context, name string) error {
	return m.CompRepo.PurgeComponentByName(name)
}

type planOutput struct {
	Timestamp        time.Time `toml:"timestamp"`
	Plan             []string  `toml:"plan"`
	ChangedResources []change
}

type change struct {
	ResourceName, Before, After string
}

func (m *Materia) SavePlan(p *Plan, outputfile string) error {
	path := filepath.Join(m.OutputDir, outputfile)
	planOutput := planOutput{
		Timestamp: time.Now(),
		Plan:      p.PrettyLines(),
	}
	for _, a := range p.Steps() {
		if a.Todo == ActionUpdate {
			diffs := a.Content.([]diffmatchpatch.Diff)
			dmp := diffmatchpatch.New()
			before := dmp.DiffText1(diffs)
			after := dmp.DiffText2(diffs)
			planOutput.ChangedResources = append(planOutput.ChangedResources, change{
				ResourceName: a.Target.Path,
				Before:       before,
				After:        after,
			})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	err = toml.NewEncoder(file).Encode(planOutput)
	if err != nil {
		return fmt.Errorf("failed to encode plan to TOML: %w", err)
	}
	return nil
}
