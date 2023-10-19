package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	"github.com/ultraviolet-black/cruiser/pkg/server"
	"github.com/ultraviolet-black/cruiser/pkg/state"
	"google.golang.org/grpc"
)

var (
	xdsGrpcServer *grpc.Server
	xdsServer     server.Server

	xdsCmd = &cobra.Command{
		Use:   "xds",
		Short: "xDS is the control plane for Envoy proxy",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			root := cmd
			for ; root.HasParent(); root = root.Parent() {
			}
			if err := root.PersistentPreRunE(cmd, args); err != nil {
				return err
			}

			xdsGrpcServer = grpc.NewServer()

			xdsServer = server.NewServer(
				server.WithListenerAddress(listenerAddress),
				server.WithShutdownTimeout(shutdownTimeout),
				server.WithListenerProtocol(listenerProtocol),
				server.WithHTTPHandler(xdsGrpcServer),
				server.WithTLSConfig(tlsContext),
			)

			stateManager = state.NewStateManager(
				state.WithTfstateSource(tfstateSource),
				state.WithPeriodicSyncInterval(periodSyncInterval),
				state.WithXdsState(),
			)

			return nil

		},
	}

	xdsStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the xDS server",
		RunE: func(cmd *cobra.Command, args []string) error {

			stateManager.XdsState().Register(cmd.Context(), xdsGrpcServer)

			go stateManager.Start(cmd.Context())

			go func() {
				for {
					select {

					case <-cmd.Context().Done():

						observability.Log.Info("xDS: context cancelled")
						return

					case err := <-stateManager.ErrorCh():

						observability.Log.Error(err.Error())

						signalCh <- os.Kill
						return

					case <-stateManager.UpdateXdsCh():

						observability.Log.Debug("xDS update received")

					}
				}
			}()

			if err := xdsServer.Open(); err != nil {
				return err
			}

			waitClose.Wait()

			return nil
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {

			xdsGrpcServer.GracefulStop()

			return xdsServer.Close()

		},
	}
)
