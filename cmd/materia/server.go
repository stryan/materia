package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/source"
	"primamateria.systems/materia/pkg/sourceman"
)

type ServerConfig struct {
	PlanInterval, UpdateInterval int    `koanf:"plan_interval" toml:"plan_interval"`
	Hostname                     string `koanf:"hostname" toml:"hostname"`
	NotifyWebhook                string `koanf:"notify_webhook" toml:"notify_webhook"`
	UpdateWebhook                bool   `koanf:"update_webhook" toml:"update_webhook"`
	UpdateUrl                    string `koanf:"sync_url" toml:"sync_url"`
	UpdateSecret                 string `koanf:"sync_secret" toml:"sync_secret"`
	Socket                       string `koanf:"socket" toml:"socket"`
}

type Server struct {
	NotifyWebhook                string
	syncSecret                   string
	Socket                       string
	UpdateInterval, PlanInterval int
	QuitOnError                  bool
	materia                      *materia.Materia
}

func (c ServerConfig) Validate() error {
	return nil
}

func NewConfig(k *koanf.Koanf) (*ServerConfig, error) {
	var c ServerConfig
	c.UpdateInterval = k.Int("server.update_interval")
	c.PlanInterval = k.Int("server.plan_interval")
	c.NotifyWebhook = k.String("server.notify_webhook")
	c.UpdateWebhook = k.Bool("server.sync_webhook")
	c.UpdateSecret = k.String("server.sync_secret")
	c.UpdateUrl = k.String("server.sync_url")
	c.Socket = k.String("server.socket")
	return &c, nil
}

func serverMateria(ctx context.Context, k *koanf.Koanf) (*materia.Materia, error) {
	c, err := materia.NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	err = c.Validate()
	if err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	if err := setupDirectories(c); err != nil {
		return nil, fmt.Errorf("error creating base directories: %w", err)
	}

	mainRepo, err := getLocalRepo(k, c.SourceDir)
	if err != nil {
		return nil, err
	}
	hmc := &hostman.HostmanConfig{
		Hostname:         c.Hostname,
		DataDir:          c.MateriaDir,
		QuadletDir:       c.QuadletDir,
		ScriptsDir:       c.ScriptsDir,
		ServicesDir:      c.ServiceDir,
		ServicesConfig:   c.ServicesConfig,
		ContainersConfig: c.ContainersConfig,
	}
	smc := &sourceman.SourceManConfig{
		SourceDir: c.SourceDir,
		RemoteDir: c.RemoteDir,
	}
	sm, err := sourceman.NewSourceManager(smc)
	if err != nil {
		return nil, err
	}
	err = sm.AddSource(mainRepo, nil)
	if err != nil {
		return nil, err
	}
	err = sm.Sync(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error with initial repo sync: %w", err)
	}
	err = sm.LoadRemotes(ctx)
	if err != nil {
		return nil, fmt.Errorf("error with repo remotes sync: %w", err)
	}

	hm, err := hostman.NewHostManager(ctx, hmc)
	if err != nil {
		return nil, err
	}
	m, err := materia.NewMateriaFromConfig(ctx, c, hm, sm)
	if err != nil {
		log.Fatal(err)
	}
	return m, nil
}

func RunServer(ctx context.Context, k *koanf.Koanf) error {
	ctx, serverClose := context.WithCancel(ctx)
	defer serverClose()
	log.Info("Starting server mode")
	conf, err := NewConfig(k)
	if err != nil {
		return err
	}
	if err := conf.Validate(); err != nil {
		return err
	}
	if conf.NotifyWebhook != "" {
		log.Infof("starting up with notify webhook %v", conf.NotifyWebhook)
	}

	m, err := serverMateria(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia server: %w", err)
		}
	}()

	log.Info("Materia instance created")
	serv := &Server{
		NotifyWebhook:  conf.NotifyWebhook,
		syncSecret:     conf.UpdateSecret,
		Socket:         conf.Socket,
		PlanInterval:   conf.PlanInterval,
		UpdateInterval: conf.UpdateInterval,
		materia:        m,
	}
	spath := serv.Socket
	if spath == "" {
		spath, err = socketPath()
		if err != nil {
			return err
		}
		spath = "unix:" + spath
	}
	vserv, err := newVarlinkServer(ctx, m)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Info("trying to shutdown cleanly")
		serverClose()
		err = vserv.Shutdown()
		if err != nil {
			log.Warn("error closing socket", "error", err)
		}
	}()
	if conf.UpdateInterval > 0 {
		wg.Add(1)
		go func() {
			log.Info("Starting background update")
			defer wg.Done()
			err = serv.backgroundSync(ctx)
			if err != nil {
				log.Fatal(err)
			}
			log.Debug("shutdown background update")
		}()
	} else {
		log.Info("skipping background update since no timer is configured")
	}
	if conf.PlanInterval != 0 {
		wg.Add(1)
		go func() {
			log.Info("Starting background plan validation")
			defer wg.Done()
			err = serv.backgroundPlan(ctx)
			if err != nil {
				log.Fatal(err)
			}
			log.Debug("shutdown background plan")
		}()

	} else {
		log.Info("skipping background plan since no timer is configured")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof("starting to listen on socket %v", spath)
		if err := vserv.Listen(ctx, spath, 0); err != nil {
			log.Fatal(err)
		}
		log.Debug("shutdown socket")
	}()
	if err := serv.notify(ctx, "server started"); err != nil {
		return err
	}
	if conf.UpdateWebhook {
		url := conf.UpdateUrl
		if url == "" {
			url = ":6284"
		}
		wg.Add(1)
		go func() {
			log.Info("starting with update webhook")
			http.HandleFunc("/webhook", serv.updateHookHandler)
			err := http.ListenAndServe(url, nil)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
	wg.Wait()

	return nil
}

func (s *Server) backgroundSync(ctx context.Context) error {
	log.Info("executing background sync")
	ticker := time.NewTicker(time.Duration(s.UpdateInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := s.materia.Source.Sync(ctx, nil)
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("Execution failed to sync sources: %v", err)); nerr != nil {
					return fmt.Errorf("execution failed to sync sources %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			plan, err := s.materia.Plan(ctx)
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("Execution failed to generate plan: %v", err)); nerr != nil {
					return fmt.Errorf("execution failed to generate plan %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			steps, err := s.materia.Execute(ctx, plan)
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("Execution failed: %v, %v/%v steps completed", err, steps, plan.Size())); nerr != nil {
					return fmt.Errorf("execution failed %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			err = s.materia.SavePlan(plan, "lastrun.toml")
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("failed to save lastrun: %v", err)); nerr != nil {
					return fmt.Errorf("last run saving failed %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			if steps == -1 {
				log.Info("Sync ran; no changes made")
			} else {
				log.Infof("Sync ran; Steps completed: %v", steps)
			}
		}
	}
}

func (s *Server) backgroundPlan(ctx context.Context) error {
	log.Info("generating plan for validation")
	ticker := time.NewTicker(time.Duration(s.PlanInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			plan, err := s.materia.Plan(ctx)
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("invalid plan: %v", err)); nerr != nil {
					return fmt.Errorf("plan generation failed with %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			log.Info("Plan generated succesfully: %v changes", plan.Size())
		}
	}
}

type hookPayload struct {
	Text string `json:"text"`
}

func (s *Server) notify(ctx context.Context, msg string) error {
	if s.NotifyWebhook == "" {
		return nil
	}
	marshaledPayload, err := json.Marshal(hookPayload{fmt.Sprintf("%v: %v", s.materia.Hostname, msg)})
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.NotifyWebhook, bytes.NewBuffer(marshaledPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	return nil
}

type UpdatePayload struct {
	Revision string
	Update   bool
	Secret   string
}

func (s *Server) updateHookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var payload UpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload json", http.StatusBadRequest)
		return
	}
	if s.syncSecret != "" && s.syncSecret != payload.Secret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	opts := &source.SyncOpts{
		Revision: payload.Revision,
	}
	w.WriteHeader(http.StatusOK)
	ctx := context.Background()
	err := s.materia.Source.Sync(ctx, opts)
	if err != nil {
		if nerr := s.notify(ctx, fmt.Sprintf("Execution failed to sync sources: %v", err)); nerr != nil {
			log.Warnf("execution failed to sync sources %v; plus the notification failed: %v", err, nerr)
		}
		if s.QuitOnError {
			log.Fatal("quitting...")
		}
		return
	}
	plan, err := s.materia.Plan(ctx)
	if err != nil {
		if nerr := s.notify(ctx, fmt.Sprintf("Execution failed to generate plan: %v", err)); nerr != nil {
			log.Warnf("execution failed to generate plan %v; plus the notification failed: %v", err, nerr)
		}
		if s.QuitOnError {
			log.Fatal("quitting...")
		}
		return
	}
	steps, err := s.materia.Execute(ctx, plan)
	if err != nil {
		if nerr := s.notify(ctx, fmt.Sprintf("Execution failed: %v, %v/%v steps completed", err, steps, plan.Size())); nerr != nil {
			log.Warnf("execution failed %v; plus the notification failed: %v", err, nerr)
		}
		if s.QuitOnError {
			log.Fatal("quitting...")
		}
		return
	}
	err = s.materia.SavePlan(plan, "lastrun.toml")
	if err != nil {
		if nerr := s.notify(ctx, fmt.Sprintf("failed to save lastrun: %v", err)); nerr != nil {
			log.Warnf("last run saving failed %v; plus the notification failed: %v", err, nerr)
		}
		if s.QuitOnError {
			log.Fatal("quitting...")
		}
		return
	}
	if steps == -1 {
		log.Info("Update ran; no changes made")
	} else {
		log.Infof("Update ran; Steps completed: %v", steps)
	}
}
