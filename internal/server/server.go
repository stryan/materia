package server

import (
	"context"
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
	"primamateria.systems/materia/pkg/facts"
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
	cancel                       context.CancelCauseFunc
	mu                           sync.Mutex
	materiaBuilder               func() (*materia.Materia, error)
	hostname                     string // TODO remove
	notifier                     *notify.Notifier
}

func getHostname(ctx context.Context, m *materia.Materia) (string, error) {
	f, err := facts.Collect(ctx, m.Collectors...)
	if err != nil {
		return "", err
	}
	mf, err := materia.ToMateriaFacts(f)
	if err != nil {
		return "", err
	}
	return mf.Hostname, nil
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
	nc, err := notify.NewConfig(k)
	if err != nil {
		return err
	}
	if conf.NotifyWebhook != "" {
		log.Warn("Server notifications config is deprecated, use the new [notify] config")
		log.Infof("starting up with notify webhook %v", conf.NotifyWebhook)
		nc = notify.NotifyConfig{
			Triggers: map[string]string{notify.NotifyUpdate: conf.NotifyWebhook},
		}
	}
	notifier, err := notify.NewNotifier(nc)
	if err != nil {
		return err
	}

	hostname, err := func() (string, error) {
		m, err := serverMateria(ctx, k, conf)
		if err != nil {
			return "", err
		}
		defer func() { _ = m.Close() }()
		return getHostname(ctx, m)
	}()
	if err != nil {
		return err
	}

	serv := &Server{
		syncSecret:     conf.UpdateSecret,
		Socket:         conf.Socket,
		PlanInterval:   conf.PlanInterval,
		UpdateInterval: conf.UpdateInterval,
		QuitOnError:    conf.QuitOnError,
		token:          conf.Token,
		cancel:         serverClose,
		materiaBuilder: func() (*materia.Materia, error) {
			return serverMateria(ctx, k, conf)
		},
		notifier: notifier,
		hostname: hostname,
	}
	spath := serv.Socket
	if spath == "" {
		return errors.New("no socket provided, unable to generate one")
	}
	vserv, err := NewVarlinkServer(ctx, serv, version)
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

	if conf.PlanInterval > 0 {
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
			if conf.FactsEndpoint {
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

func (s *Server) plan(ctx context.Context, m *materia.Materia) (*plan.Plan, error) {
	aplan, err := commands.RunPlan(ctx, m, nil)
	if aplan != nil {
		serr := m.SavePlan(ctx, aplan)
		if serr != nil {
			log.Warnf("unable to save execution plan: %v", serr)
		}
	}
	return aplan, err
}

// reused between background and foreground sync
func (s *Server) update(ctx context.Context, opts *source.SyncOpts) (steps, total int64, err error) {
	err = s.runner(func(m *materia.Materia) error {
		if err := m.Source.Sync(ctx, opts); err != nil {
			return fmt.Errorf("unable to sync sources: %w", err)
		}
		plan, err := s.plan(ctx, m)
		if err != nil {
			return fmt.Errorf("unable to generate plan: %w", err)
		}
		if plan.Empty() {
			log.Info("Update skipped; no changes planned.")
			return nil
		}
		log.Info("Executing the following plan: %v")
		for _, step := range plan.PrettyLines() {
			log.Info(step)
		}
		// last chance to bail cleanly
		if err := ctx.Err(); err != nil {
			return err
		}
		finalPlan, rep, err := commands.RunExecute(context.WithoutCancel(ctx), m, plan)
		steps, total = int64(max(rep.StepsCompleted, 0)), int64(rep.TotalSteps)
		switch {
		case err != nil:
			return fmt.Errorf("unable to execute, %v/%v steps completed: %w", steps, total, err)
		case finalPlan != plan:
			return fmt.Errorf("update rolled back, %v/%v steps completed", steps, total)
		default:
			log.Info("Update executed: %v/%v steps completed", steps, total)
		}
		return nil
	})
	return
}

func (s *Server) backgroundUpdate(ctx context.Context) error {
	log.Info("executing background update loop", "interval", s.UpdateInterval)
	ticker := time.NewTicker(time.Duration(s.UpdateInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			log.Info("Running background update")
			_, _, err := s.update(ctx, nil)
			if err != nil {
				log.Warnf("background update failed: %v", err)
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
			log.Info("Running background plan")
			cerr := s.runner(func(m *materia.Materia) error {
				plan, err := s.plan(ctx, m)
				if err != nil {
					return err
				}
				log.Infof("Plan generated successfully: %v changes", plan.Size())
				return nil
			})
			if cerr != nil {
				log.Warnf("plan generation failed: %v", cerr)
				if nerr := s.notifyOnFailure(ctx, "failed to generate plan", cerr); nerr != nil {
					return nerr
				}
			}
		}
	}
}

func (s *Server) notify(ctx context.Context, msg string) error {
	payload := fmt.Sprintf("%v: %v", s.hostname, msg)
	return s.notifier.Notify(ctx, notify.NotifyDefault, payload)
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
