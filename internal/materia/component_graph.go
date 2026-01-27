package materia

import (
	"errors"
	"fmt"
)

var ErrTreeNotFound = errors.New("tree not found")

type ComponentGraph struct {
	graph map[string]*ComponentTree
}

func NewComponentGraph() *ComponentGraph {
	return &ComponentGraph{
		graph: make(map[string]*ComponentTree),
	}
}

func (g *ComponentGraph) Add(tree *ComponentTree) error {
	if tree.Source != nil && tree.Host != nil && tree.Source.Name != tree.Host.Name {
		return fmt.Errorf("tried to add two seperate components as tree: %v vs %v", tree.Source.Name, tree.Host.Name)
	}
	if tree.Source == nil && tree.Host == nil {
		return fmt.Errorf("tried to add empty component tree: %v", tree.Name)
	}
	if tree.Name == "" {
		return fmt.Errorf("tried to add unnamed component")
	}
	g.graph[tree.Name] = tree
	return nil
}

func (g *ComponentGraph) Get(name string) (*ComponentTree, error) {
	if tree, ok := g.graph[name]; !ok {
		return nil, ErrTreeNotFound
	} else {
		return tree, nil
	}
}

func (g *ComponentGraph) List() []*ComponentTree {
	var result []*ComponentTree
	allComponentNames := sortedKeys(g.graph)
	for _, name := range allComponentNames {
		result = append(result, g.graph[name])
	}
	return result
}
