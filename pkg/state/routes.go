package state

import (
	"sync"

	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"
	"google.golang.org/protobuf/encoding/protojson"
)

type RoutesState interface {
	GetRoutes() ([]*serverpb.Router_Route, error)
}

type routesState struct {
	routes    Graph[*serverpb.Router_Route]
	routesMap map[string]*serverpb.Router_Route

	rwLock *sync.RWMutex
}

func readRoutesFromTfstate(tfstate *Tfstate, rs *routesState) error {

	for _, resource := range tfstate.Resources {

		if resource.Type != "cruiser_route" {
			continue
		}

		for _, instance := range resource.Instances {

			route := &serverpb.Router_Route{}

			if err := protojson.Unmarshal(instance, route); err != nil {
				return err
			}

			rs.routesMap[route.Name] = route

		}

	}

	return nil

}

func (r *routesState) build() error {

	r.rwLock.Lock()
	defer r.rwLock.Unlock()

	for _, route := range r.routesMap {

		if route.ParentName == "" {
			r.routes.AddSingleNode(route)
			continue
		}

		parent, ok := r.routesMap[route.ParentName]
		if !ok {
			return ErrNoParentFound
		}

		r.routes.AddEdge(route, parent)

	}

	r.routesMap = make(map[string]*serverpb.Router_Route)

	return nil

}

func (r *routesState) GetRoutes() ([]*serverpb.Router_Route, error) {

	r.rwLock.RLock()
	defer r.rwLock.RUnlock()

	return r.routes.TopologicalSort()

}
