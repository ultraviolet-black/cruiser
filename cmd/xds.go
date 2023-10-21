package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	servicediscovery "github.com/ultraviolet-black/cruiser/pkg/providers/aws/service_discovery"
	"github.com/ultraviolet-black/cruiser/pkg/server"
	"github.com/ultraviolet-black/cruiser/pkg/state"
	"google.golang.org/grpc"
)

var (
	xdsGrpcServer *grpc.Server
	xdsServer     server.Server

	xdsState state.XdsState

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

			xdsState = state.NewXdsState()

			stateManager = state.NewStateManager(
				state.WithTfstateSource(tfstateSource),
				state.WithPeriodicSyncInterval(periodSyncInterval),
				state.WithManagers(xdsState),
			)

			if len(awsServiceDiscoveryNamespaces) > 0 {

				awsServiceDiscoveryXds = servicediscovery.NewXds(
					servicediscovery.WithNamespacesNames(awsServiceDiscoveryNamespaces...),
					servicediscovery.WithServicePortTagKey(awsServicePortTagKey),
					servicediscovery.WithPeriodicSyncInterval(periodSyncInterval),
					servicediscovery.WithServiceDiscoveryClient(awsProvider.GetServiceDiscoveryClient()),
					servicediscovery.WithXdsState(xdsState),
				)

				go func() {

					select {
					case err := <-awsServiceDiscoveryXds.ErrorCh():

						observability.Log.Error("aws service discovery error:", "error", err)

						signalCh <- os.Kill
						return
					}

				}()

				awsServiceDiscoveryXds.Start(cmd.Context())

			}

			return nil

		},
	}

	xdsStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the xDS server",
		RunE: func(cmd *cobra.Command, args []string) error {

			xdsState.Register(cmd.Context(), xdsGrpcServer)

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

					case <-xdsState.UpdateCh():

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
