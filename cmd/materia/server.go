package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
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
	quit                         chan any
	materia                      *materia.Materia
}

func (c ServerConfig) Validate() error {
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
		quit:           make(chan any),
		materia:        m,
	}
	socket, path, err := serv.setupSocket()
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	defer func() {
		_ = socket.Close()
	}()
	var wg sync.WaitGroup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Info("trying to shutdown cleanly")
		serverClose()
		close(serv.quit)
		err = socket.Close()
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
		}()

	} else {
		log.Info("skipping background plan since no timer is configured")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof("starting to listen on socket %v", path)
		err := serv.listenForCommands(socket)
		if err != nil {
			log.Fatal(err)
		}
	}()
	if err := serv.notify(ctx, "server started"); err != nil {
		return err
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
				if nerr := s.notify(ctx, fmt.Sprintf("Execution failed: %v, %v/%v steps completed", err, steps, len(plan.Steps()))); nerr != nil {
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

func (s *Server) setupSocket() (net.Listener, string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, "", err
	}
	socketDir := ""
	socketPath := ""
	if s.Socket == "" {
		if currentUser.Name != "root" {
			uid := currentUser.Uid
			socketDir = filepath.Join("/run/user", uid, "materia")
		} else {
			socketDir = filepath.Join("/run/materia")
		}
		err = os.MkdirAll(socketDir, 0o700)
		if err != nil {
			return nil, "", err
		}
		socketPath = filepath.Join(socketDir, "materia.sock")
	}
	sock, err := net.Listen("unix", socketPath)
	return sock, socketPath, err
}

func (s *Server) listenForCommands(sock net.Listener) error {
	for {
		conn, err := sock.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return nil
			default:
				log.Fatal(err)
			}
		}

		go func() {
			log.Debug("parsing command")
			err := func(conn net.Conn) error {
				defer func() {
					err := conn.Close()
					if err != nil {
						log.Warn("err closing command socket ", "error", err)
					}
				}()

				var msg SocketMessage
				if err := json.NewDecoder(conn).Decode(&msg); err != nil {
					return err
				}

				resp, err := s.parseCommand(msg)
				if err != nil {
					return err
				}
				jsonBytes, err := json.Marshal(resp)
				if err != nil {
					return err
				}
				_, err = conn.Write(jsonBytes)
				if err != nil {
					return err
				}
				return nil
			}(conn)
			if err != nil {
				log.Warn(err)
			}
		}()
	}
}

func (s *Server) parseCommand(cmd SocketMessage) (SocketMessage, error) {
	ctx := context.Background()
	switch cmd.Name {
	case "facts":
		log.Info("generating facts on request")
		return SocketMessage{Name: "result", Data: s.materia.GetFacts(false)}, nil
	case "plan":
		log.Info("generating a plan on request")
		plan, err := s.materia.Plan(context.Background())
		if err != nil {
			return errToMsg(err), nil
		}
		resp := "no changes made"
		if plan.Size() != 0 {
			data, err := plan.ToJson()
			if err != nil {
				return errToMsg(err), nil
			}
			resp = string(data)
		}
		return SocketMessage{Name: "result", Data: resp}, nil
	case "update":
		log.Info("running update on request")
		plan, err := s.materia.Plan(ctx)
		if err != nil {
			return errToMsg(err), nil
		}
		_, err = s.materia.Execute(ctx, plan)
		if err != nil {
			return errToMsg(err), nil
		}
		return SocketMessage{Name: "result", Data: "success"}, nil
	case "sync":
		log.Info("syncing local source on request")
		err := s.materia.Source.Sync(ctx)
		if err != nil {
			return errToMsg(err), nil
		}
		return SocketMessage{Name: "result", Data: "success"}, nil
	default:
		return SocketMessage{Name: "result", Data: "command not found"}, nil
	}
}
