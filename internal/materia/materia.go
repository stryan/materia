package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"

	"git.saintnet.tech/stryan/materia/internal/components"
	"git.saintnet.tech/stryan/materia/internal/containers"
	fprov "git.saintnet.tech/stryan/materia/internal/facts"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	filesecrets "git.saintnet.tech/stryan/materia/internal/secrets/file"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/services"
	"git.saintnet.tech/stryan/materia/internal/source"
	"git.saintnet.tech/stryan/materia/internal/source/file"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

type MacroMap func(map[string]any) template.FuncMap

type Materia struct {
	Facts         fprov.FactsProvider
	Manifest      *manifests.MateriaManifest
	Services      services.Services
	PodmanConn    context.Context
	Containers    containers.ContainerManager
	Secrets       secrets.SecretsManager
	source        source.Source
	CompRepo      repository.ComponentRepository
	ScriptRepo    repository.Repository
	ServiceRepo   repository.Repository
	SourceRepo    repository.ComponentRepository
	rootComponent *components.Component
	macros        MacroMap
	snippets      map[string]*Snippet
	OutputDir     string
	onlyResources bool
	debug         bool
	diffs         bool
	cleanup       bool
}

func NewMateria(ctx context.Context, c *Config, sm services.Services, cm containers.ContainerManager, scriptRepo, serviceRepo repository.Repository, sourceRepo repository.ComponentRepository, hostRepo repository.ComponentRepository) (*Materia, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(c.QuadletDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("destination %v does not exist, setup manually", c.QuadletDir)
	}
	if _, err := os.Stat(c.ScriptsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("scripts location %v does not exist, setup manually", c.ScriptsDir)
	}
	if _, err := os.Stat(c.ServiceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("services location %v does not exist, setup manually", c.ServiceDir)
	}

	err := os.Mkdir(filepath.Join(c.MateriaDir, "materia"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(c.OutputDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating output dir: %w", err)
	}
	err = os.Mkdir(c.SourceDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(c.MateriaDir, "materia", "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating components in prefix: %w", err)
	}

	var source source.Source
	parsedPath := strings.Split(c.SourceURL, "://")
	switch parsedPath[0] {
	case "git":
		source, err = git.NewGitSource(c.SourceDir, parsedPath[1], c.GitConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		source = file.NewFileSource(c.SourceDir, parsedPath[1])
	default:
		return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
	}

	// Ensure local cache
	if c.NoSync {
		log.Debug("skipping cache update on request")
	} else {
		log.Debug("updating configured source cache")
		err = source.Sync(ctx)
		if err != nil {
			return nil, fmt.Errorf("error syncing source: %w", err)
		}
	}
	log.Debug("loading manifest")
	man, err := manifests.LoadMateriaManifest(filepath.Join(c.SourceDir, "MANIFEST.toml"))
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, err
	}

	log.Debug("loading facts")
	facts, err := fprov.NewFacts(ctx, c.Hostname, man, hostRepo, cm)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}
	snips := make(map[string]*Snippet)
	defaultSnippets := loadDefaultSnippets()
	for _, v := range defaultSnippets {
		snips[v.Name] = v
	}
	m := &Materia{
		Services:      sm,
		Containers:    cm,
		Facts:         facts,
		Manifest:      man,
		source:        source,
		debug:         c.Debug,
		diffs:         c.Diffs,
		cleanup:       c.Cleanup,
		onlyResources: c.OnlyResources,
		CompRepo:      hostRepo,
		ScriptRepo:    scriptRepo,
		ServiceRepo:   serviceRepo,
		SourceRepo:    sourceRepo,
		OutputDir:     c.OutputDir,
		snippets:      snips,
		rootComponent: &components.Component{Name: "root"},
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
				return m.Facts.Lookup(arg)
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

	switch m.Manifest.Secrets {
	case "age":
		conf, ok := m.Manifest.SecretsConfig.(*age.Config)
		if !ok {
			return nil, errors.New("tried to create an age secrets manager but config was not for age")
		}
		if c.AgeConfig != nil {
			conf.Merge(c.AgeConfig)
		}
		m.Secrets, err = age.NewAgeStore(*conf, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}
	case "file":
		conf, ok := m.Manifest.SecretsConfig.(*filesecrets.Config)
		if !ok {
			return nil, errors.New("tried to create an file secrets manager but config was not for file")
		}
		if c.FileConfig != nil {
			conf.Merge(c.FileConfig)
		}
		m.Secrets, err = filesecrets.NewFileStore(*conf, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating file store: %w", err)
		}

	case "mem":
		m.Secrets = mem.NewMemoryManager()
	default:
		// TODO allow this to be empty if secrets config is passed at runtime via config/env
		return nil, errors.New("unknown secrets config in manifest")
	}
	for _, v := range m.Manifest.Snippets {
		s, err := configToSnippet(v)
		if err != nil {
			return nil, err
		}
		m.snippets[s.Name] = s
	}
	return m, nil
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
	return nil
}

func (m *Materia) CleanComponent(ctx context.Context, name string) error {
	isInstalled := slices.Contains(m.Facts.GetInstalledComponents(), name)
	if !isInstalled {
		return errors.New("component not installed")
	}
	comp, err := m.CompRepo.GetComponent(name)
	if err != nil {
		return err
	}
	emptyVars := make(map[string]any)
	for _, r := range comp.Resources {
		err := m.executeAction(ctx, Action{
			Todo:    resToAction(r, "remove"),
			Parent:  comp,
			Payload: r,
		}, emptyVars)
		if err != nil {
			return err
		}
	}
	return m.CompRepo.RemoveComponent(comp)
}

func (m *Materia) PlanComponent(ctx context.Context, name string, roles []string) (*Plan, error) {
	// if roles != nil {
	// 	m.Facts.Roles = roles
	// }
	// if name != "" {
	// 	m.Facts.AssignedComponents = []string{name}
	// }
	// m.Services = &services.PlannedServiceManager{}
	// m.Facts.InstalledComponents = make(map[string]*components.Component)
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

func (m *Materia) SavePlan(p *Plan, outputfile string) error {
	path := filepath.Join(m.OutputDir, outputfile)
	planOutput := struct {
		Timestamp time.Time `toml:"timestamp"`
		Plan      []string  `toml:"plan"`
	}{
		Timestamp: time.Now(),
		Plan:      p.PrettyLines(),
	}

	// Create or truncate the output file
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
