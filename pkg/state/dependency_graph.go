package state

import (
	"container/list"
)

type graph[T any] struct {
	nodes     map[string]*node[T]
	keyGetter func(T) string
}

type node[T any] struct {
	val          T
	dependencies []*node[T]
}

type Graph[T any] interface {
	AddSingleNode(val T)
	AddEdge(from, to T)
	TopologicalSort() ([]T, error)
}

func NewGraph[T any](keyGetter func(T) string) Graph[T] {
	return &graph[T]{
		nodes:     make(map[string]*node[T]),
		keyGetter: keyGetter,
	}
}

func (g *graph[T]) addNode(val T) *node[T] {
	key := g.keyGetter(val)

	if _, exists := g.nodes[key]; !exists {
		g.nodes[key] = &node[T]{val: val, dependencies: []*node[T]{}}
	}
	return g.nodes[key]
}

func (g *graph[T]) AddSingleNode(val T) {
	g.addNode(val)
}

func (g *graph[T]) AddEdge(from, to T) {
	fromNode := g.addNode(from)
	toNode := g.addNode(to)
	fromNode.dependencies = append(fromNode.dependencies, toNode)
}

func (g *graph[T]) TopologicalSort() ([]T, error) {
	result := []T{}
	visited := make(map[string]bool)
	l := list.New()

	var visit func(n *node[T]) error

	visit = func(n *node[T]) error {
		key := g.keyGetter(n.val)

		if _, ok := visited[key]; ok {
			return nil
		}
		visited[key] = true
		for _, dependency := range n.dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}

		l.PushFront(n)

		return nil
	}

	for _, node := range g.nodes {

		key := g.keyGetter(node.val)

		if _, ok := visited[key]; !ok {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}

	for e := l.Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(T))
	}

	return result, nil

}
