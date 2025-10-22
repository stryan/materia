package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/sourceman"
)

type ServerConfig struct {
	PlanInterval, UpdateInterval int
	Webhook                      string
	Socket                       string
}

type Server struct {
	hostname                     string
	Webhook                      string
	Socket                       string
	UpdateInterval, PlanInterval int
	QuitOnError                  bool
}

func (c ServerConfig) Validate() error {
	if c.UpdateInterval <= 1 {
		return errors.New("need to at least set an update interval")
	}
	return nil
}

func NewConfig(k *koanf.Koanf) (*ServerConfig, error) {
	var c ServerConfig
	c.UpdateInterval = k.Int("server.update_interval")
	c.PlanInterval = k.Int("server.plan_interval")
	c.Webhook = k.String("server.webhook")
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
	sm, err := sourceman.NewSourceManager(c)
	if err != nil {
		return nil, err
	}
	err = sm.AddSource(mainRepo)
	if err != nil {
		return nil, err
	}
	err = sm.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("error with initial repo sync: %w", err)
	}
	err = sm.SyncRemotes(ctx)
	if err != nil {
		return nil, fmt.Errorf("error with repo remotes sync: %w", err)
	}
	hm, err := hostman.NewHostManager(c)
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
	if conf.Webhook != "" {
		log.Infof("Starting up with webhook %v", conf.Webhook)
	}

	m, err := serverMateria(ctx, k)
	if err != nil {
		return err
	}
	log.Info("Materia instance created")
	serv := &Server{
		Webhook:        conf.Webhook,
		Socket:         conf.Socket,
		PlanInterval:   conf.PlanInterval,
		UpdateInterval: conf.UpdateInterval,
		hostname:       m.Host.GetHostname(),
	}
	var wg sync.WaitGroup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Info("trying to shutdown cleanly")
		serverClose()
	}()
	wg.Add(1)
	go func() {
		log.Info("Starting background sync")
		defer wg.Done()
		err = serv.backgroundSync(ctx, m)
		if err != nil {
			log.Fatal(err)
		}
	}()
	if conf.PlanInterval != 0 {
		wg.Add(1)
		go func() {
			log.Info("Starting background plan validation")
			defer wg.Done()
			err = serv.backgroundPlan(ctx, m)
			if err != nil {
				log.Fatal(err)
			}
		}()

	}
	if err := serv.notify(ctx, "server started"); err != nil {
		return err
	}
	wg.Wait()

	return nil
}

func (s *Server) backgroundSync(ctx context.Context, m *materia.Materia) error {
	log.Info("executing background sync")
	ticker := time.NewTicker(time.Duration(s.UpdateInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			plan, err := m.Plan(ctx)
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("Execution failed to generate plan: %v", err)); nerr != nil {
					return fmt.Errorf("execution failed to generate plan %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			steps, err := m.Execute(ctx, plan)
			if err != nil {
				if nerr := s.notify(ctx, fmt.Sprintf("Execution failed: %v, %v/%v steps completed", err, steps, len(plan.Steps()))); nerr != nil {
					return fmt.Errorf("execution failed %w; plus the notification failed: %w", err, nerr)
				}
				if s.QuitOnError {
					return err
				}
				break
			}
			err = m.SavePlan(plan, "lastrun.toml")
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

func (s *Server) backgroundPlan(ctx context.Context, m *materia.Materia) error {
	log.Info("generating plan for validation")
	ticker := time.NewTicker(time.Duration(s.PlanInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			plan, err := m.Plan(ctx)
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
	if s.Webhook == "" {
		return nil
	}
	marshaledPayload, err := json.Marshal(hookPayload{fmt.Sprintf("%v: %v", s.hostname, msg)})
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Webhook, bytes.NewBuffer(marshaledPayload))
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
