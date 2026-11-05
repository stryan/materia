package server

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/commands"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/notify"
)

func (s *Server) runner(fn func(*materia.Materia) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	smat, err := s.materiaBuilder()
	if err != nil {
		return fmt.Errorf("unable to create materia instance: %w", err)
	}
	defer func() {
		err := smat.Close()
		if err != nil {
			log.Warnf("error closing materia instance: %v", err)
		}
	}()
	return fn(smat)
}

func serverMateria(ctx context.Context, k *koanf.Koanf, sc *ServerConfig) (*materia.Materia, error) {
	c, err := materia.NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	materia.SetupLogger(c)

	if sc.NotifyWebhook != "" {
		c.NotifyConfig = notify.NotifyConfig{
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
