package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"charm.land/log/v2"
	"primamateria.systems/materia/internal/commands"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/source"
)

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
			_, _, err := s.update(ctx, opts)
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
		var resFacts *facts.Facts
		err := s.runner(func(m *materia.Materia) error {
			var err error
			resFacts, err = commands.RunFacts(ctx, m)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			http.Error(w, "unable to generate facts", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resFacts); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
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
