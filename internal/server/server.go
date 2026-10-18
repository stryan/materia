package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"golang.org/x/sync/errgroup"
	"primamateria.systems/materia/internal/commands"
	"primamateria.systems/materia/internal/rpc"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/notify"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/source"
)

type Server struct {
	syncSecret                   string
	token                        string
	Socket                       string
	UpdateInterval, PlanInterval int
	QuitOnError                  bool
	materia                      *materia.Materia
	cancel                       context.CancelCauseFunc
	mu                           sync.Mutex
}

func serverMateria(ctx context.Context, k *koanf.Koanf, sc *ServerConfig) (*materia.Materia, error) {
	c, err := materia.NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	materia.SetupLogger(c)

	if sc.NotifyWebhook != "" {
		c.NotifyConfig = &notify.NotifyConfig{
			Triggers: map[string]string{notify.NotifyUpdate: sc.NotifyWebhook},
		}
	}

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	src, err := commands.BuildSource(k)
	if err != nil {
		return nil, err
	}

	return materia.NewFromConfig(ctx, c, src)
}

func RunServer(ctx context.Context, k *koanf.Koanf, version string) error {
	ctx, serverClose := context.WithCancelCause(ctx)
	defer serverClose(nil)
	log.Info("Starting server mode")
	conf, err := NewConfig(k)
	if err != nil {
		return err
	}
	if err := conf.Validate(); err != nil {
		return err
	}
	if conf.NotifyWebhook != "" {
		log.Warn("Server notifications config is deprecated, use the new [notify] config")
		log.Infof("starting up with notify webhook %v", conf.NotifyWebhook)
	}

	m, err := serverMateria(ctx, k, conf)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warnf("error closing materia server: %v", err)
		}
	}()

	log.Info("Materia instance created")
	serv := &Server{
		syncSecret:     conf.UpdateSecret,
		Socket:         conf.Socket,
		PlanInterval:   conf.PlanInterval,
		UpdateInterval: conf.UpdateInterval,
		QuitOnError:    conf.QuitOnError,
		token:          conf.Token,
		cancel:         serverClose,
		materia:        m,
	}
	spath := serv.Socket
	if spath == "" {
		return errors.New("no socket provided, unable to generate one")
	}
	vserv, err := rpc.NewVarlinkServer(ctx, m, version)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-gctx.Done()
		stop()
		log.Info("trying to shutdown cleanly")
		return vserv.Shutdown()
	})

	if conf.UpdateInterval > 0 {
		g.Go(func() error { return serv.backgroundUpdate(gctx) })
	} else {
		log.Info("skipping background update since no timer is configured")
	}

	if conf.PlanInterval != 0 {
		g.Go(func() error { return serv.backgroundPlan(gctx) })
	} else {
		log.Info("skipping background plan since no timer is configured")
	}

	g.Go(func() error {
		log.Infof("starting to listen on varlink socket %v", spath)
		err := vserv.Listen(gctx, spath, 0)
		if gctx.Err() != nil { // this is a cancellation error
			return nil
		}
		return err
	})

	if conf.HasWebhook() {
		url := conf.Url
		if url == "" {
			url = ":6284"
		}
		g.Go(func() error {
			log.Info("starting webhook server")
			mux := http.NewServeMux()
			if conf.UpdateWebhook {
				authFunc := serv.validateSyncSecret()
				if conf.Token != "" {
					authFunc = serv.validateToken()
				}
				mux.Handle("/webhook", authFunc(http.HandlerFunc(serv.updateHookHandler(gctx))))
			}
			if conf.FactsWebhook {
				authFunc := serv.validateToken()
				mux.Handle("/facts", authFunc(http.HandlerFunc(serv.factsHookHandler(gctx))))
			}
			// Health is always enabled if the config has a webhook endpoint
			mux.Handle("/health", http.HandlerFunc(healthHandler))
			srv := &http.Server{
				Addr:              url,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       15 * time.Second,
				MaxHeaderBytes:    1 << 16,
			}
			go func() {
				<-gctx.Done()
				sctx, c := context.WithTimeout(context.Background(), 10*time.Second)
				defer c()
				_ = srv.Shutdown(sctx)
			}()
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			log.Info("shutdown webhook server")
			return nil
		})
	}
	if err := serv.notify(ctx, "server started"); err != nil {
		log.Warn(err)
	}
	gerr := g.Wait()
	serv.mu.Lock() // make sure we end everthing sequentially
	defer serv.mu.Unlock()
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return gerr
}

// reused between background and foreground sync
func (s *Server) execute(ctx context.Context, plan *plan.Plan) (materia.ExecutionReport, error) {
	rep, err := s.materia.Execute(ctx, plan)
	serr := s.materia.SaveExecution(ctx, plan, rep)
	if serr != nil {
		log.Warnf("unable to save execution report: %v", serr)
	}
	return rep, err
}

// reused between background and foreground sync
func (s *Server) plan(ctx context.Context) (*plan.Plan, error) {
	aplan, err := s.materia.Plan(ctx)
	serr := s.materia.SavePlan(ctx, aplan)
	if serr != nil {
		log.Warnf("unable to save execution plan: %v", serr)
	}
	return aplan, err
}

func (s *Server) update(ctx context.Context, opts *source.SyncOpts) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.materia.Source.Sync(ctx, opts)
	if err != nil {
		return fmt.Errorf("unable to sync sources: %w", err)
	}
	plan, err := s.plan(ctx)
	if err != nil {
		return fmt.Errorf("unable to generate plan: %w", err)
	}

	// last chance to bail cleanly
	if err := ctx.Err(); err != nil {
		return err
	}

	rep, err := s.execute(context.WithoutCancel(ctx), plan)
	if err != nil {
		return fmt.Errorf("unable to execute, %v/%v steps completed: %w", rep.StepsCompleted, rep.TotalSteps, err)
	}
	if rep.StepsCompleted == -1 {
		log.Info("Update ran; no changes made")
	} else {
		log.Infof("Update ran; Steps completed: %v/%v", rep.StepsCompleted, rep.TotalSteps)
	}
	return nil
}

func (s *Server) backgroundUpdate(ctx context.Context) error {
	log.Info("executing background update loop")
	ticker := time.NewTicker(time.Duration(s.UpdateInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := s.update(ctx, nil)
			if err != nil {
				if nerr := s.notifyOnFailure(ctx, "failed to update", err); nerr != nil {
					return nerr
				}
			}
		}
	}
}

func (s *Server) backgroundPlan(ctx context.Context) error {
	log.Info("generating plan for validation")
	ticker := time.NewTicker(time.Duration(s.PlanInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			plan, cerr := func() (*plan.Plan, error) {
				s.mu.Lock()
				defer s.mu.Unlock()
				return s.plan(ctx)
			}()
			if cerr != nil {
				if nerr := s.notifyOnFailure(ctx, "failed to generate plan", cerr); nerr != nil {
					return nerr
				}
				break
			}
			log.Infof("Plan generated successfully: %v changes", plan.Size())
		}
	}
}

func (s *Server) notify(ctx context.Context, msg string) error {
	payload := fmt.Sprintf("%v: %v", s.materia.Hostname, msg)
	return s.materia.Notifier.Notify(ctx, notify.NotifyDefault, payload)
}

type UpdatePayload struct {
	Revision string
	Update   bool
	Secret   string
}

func (s *Server) updateHookHandler(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var payload UpdatePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload json", http.StatusBadRequest)
			return
		}
		opts := &source.SyncOpts{
			Revision: payload.Revision,
		}
		w.WriteHeader(http.StatusAccepted)
		go func() {
			err := s.update(ctx, opts)
			if err != nil {
				if nerr := s.notifyOnFailure(ctx, "update failed", err); nerr != nil {
					s.cancel(nerr)
					return
				}
			}
		}()
	}
}

func (s *Server) factsHookHandler(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "invalid request method", http.StatusMethodNotAllowed)
			return
		}
		facts, err := commands.GetFacts(ctx, s.materia)
		if err != nil {
			http.Error(w, "unable to generate facts", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(facts); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) notifyOnFailure(ctx context.Context, what string, err error) error {
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return nil // shutting down, not a real failure
	}
	nctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if nerr := s.notify(nctx, fmt.Sprintf("%s: %v", what, err)); nerr != nil {
		log.Warnf("notification failed: %v, original error: %v", nerr, err)
	}
	if s.QuitOnError {
		return err
	}
	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "invalid request method", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	response := map[string]any{
		"health":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Warnf("health check error: %v", err)
	}
}
