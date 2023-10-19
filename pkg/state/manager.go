package state

import (
	"context"
	"time"
)

type state struct {
	xds    *xdsState
	routes *routesState

	tfstateSource TfstateSource

	periodicSyncInterval time.Duration

	errCh          chan error
	updateXdsCh    chan XdsState
	updateRoutesCh chan RoutesState
}

func (s *state) periodicSync(ctx context.Context) {

	for {

		tfstates, err := s.tfstateSource.GetTfstate(ctx)
		if err != nil {
			s.errCh <- err
			return
		}

		if tfstates != nil {

			if s.routes != nil {

				for _, tfstate := range tfstates {
					if err := readRoutesFromTfstate(tfstate, s.routes); err != nil {
						s.errCh <- err
						return
					}
				}

				if err := s.routes.build(); err != nil {
					s.errCh <- err
					return
				}

				s.updateRoutesCh <- s.routes
			}

			if s.xds != nil {

				for _, tfstate := range tfstates {
					if err := readXdsFromTfstate(tfstate, s.xds); err != nil {
						s.errCh <- err
						return
					}
				}

				if err := s.xds.build(); err != nil {
					s.errCh <- err
					return
				}

				s.updateXdsCh <- s.xds
			}

		}

		select {

		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				s.errCh <- err
			}
			return

		case <-time.After(s.periodicSyncInterval):

		}

	}

}

func (s *state) Start(ctx context.Context) {

	go s.periodicSync(ctx)

}

func (s *state) RoutesState() RoutesState {
	return s.routes
}

func (s *state) XdsState() XdsState {
	return s.xds
}

func (s *state) ErrorCh() <-chan error {
	return s.errCh
}

func (s *state) UpdateRoutesCh() <-chan RoutesState {
	return s.updateRoutesCh
}

func (s *state) UpdateXdsCh() <-chan XdsState {
	return s.updateXdsCh
}
