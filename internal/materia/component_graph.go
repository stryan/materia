package materia

import (
	"errors"
	"fmt"
)

var ErrTreeNotFound = errors.New("tree not found")

type ComponentGraph struct {
	graph map[string]*componentTree
}

func NewComponentGraph() *ComponentGraph {
	return &ComponentGraph{
		graph: make(map[string]*componentTree),
	}
}

func (g *ComponentGraph) Add(tree *componentTree) error {
	if tree.source != nil && tree.host != nil && tree.source.Name != tree.host.Name {
		return fmt.Errorf("tried to add two seperate components as tree: %v vs %v", tree.source.Name, tree.host.Name)
	}
	if tree.source == nil && tree.host == nil {
		return fmt.Errorf("tried to add empty component tree: %v", tree.Name)
	}
	if tree.Name == "" {
		return fmt.Errorf("tried to add unnamed component")
	}
	g.graph[tree.Name] = tree
	return nil
}

func (g *ComponentGraph) Get(name string) (*componentTree, error) {
	if tree, ok := g.graph[name]; !ok {
		return nil, ErrTreeNotFound
	} else {
		return tree, nil
	}
}

func (g *ComponentGraph) List() []*componentTree {
	var result []*componentTree
	allComponentNames := sortedKeys(g.graph)
	for _, name := range allComponentNames {
		result = append(result, g.graph[name])
	}
	return result
}
