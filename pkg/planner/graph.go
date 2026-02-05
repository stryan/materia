package planner

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/pkg/components"
)

type ComponentTree struct {
	Host, Source *components.Component
	Name         string
	FinalState   components.ComponentLifecycle
}

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
	allComponentNames := make([]string, 0, len(g.graph))
	for k := range g.graph {
		allComponentNames = append(allComponentNames, k)
	}
	slices.Sort(allComponentNames)

	for _, name := range allComponentNames {
		result = append(result, g.graph[name])
	}
	return result
}

func BuildComponentGraph(ctx context.Context, installedComponents, assignedComponents []*components.Component) (*ComponentGraph, error) {
	componentGraph := NewComponentGraph()

	log.Debug("loading host components")
	for _, v := range installedComponents {
		err := componentGraph.Add(&ComponentTree{
			Name: v.Name,
			Host: v,
		})
		if err != nil {
			return nil, err
		}

	}
	log.Debug("loading source components")
	for _, v := range assignedComponents {
		tree, err := componentGraph.Get(v.Name)
		if errors.Is(err, ErrTreeNotFound) {
			tree = &ComponentTree{
				Name: v.Name,
			}
		} else if err != nil {
			return nil, err
		}
		tree.Source = v
		err = componentGraph.Add(tree)
		if err != nil {
			return nil, err
		}
	}
	return componentGraph, nil
}
