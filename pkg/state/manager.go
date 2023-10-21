package state

import (
	"context"
	"sync"
	"time"
)

type Manager interface {
	ReadFromTfstate(*Tfstate) error
	Build() error
}

type state struct {
	managers []Manager

	tfstateSource TfstateSource

	periodicSyncInterval time.Duration

	errCh chan error

	wg *sync.WaitGroup
}

func (s *state) periodicSync(ctx context.Context) {

	for {

		tfstates, err := s.tfstateSource.GetTfstate(ctx)
		if err != nil {
			s.errCh <- err
			return
		}

		for _, m := range s.managers {

			s.wg.Add(1)

			go func(manager Manager) {

				for _, tfstate := range tfstates {
					if err := manager.ReadFromTfstate(tfstate); err != nil {
						s.errCh <- err
						return
					}
				}

				if err := manager.Build(); err != nil {
					s.errCh <- err
					return
				}

				s.wg.Done()

			}(m)

		}

		s.wg.Wait()

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

func (s *state) ErrorCh() <-chan error {
	return s.errCh
}
