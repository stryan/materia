package sourceman

import "primamateria.systems/materia/internal/materia"

type SourceManager struct{ materia.SourceManager }

func NewSourceManager(c *materia.MateriaConfig) (*SourceManager, error) {
	return &SourceManager{}, nil
}
