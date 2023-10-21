package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"
	servicediscovery "github.com/ultraviolet-black/cruiser/pkg/providers/aws/service_discovery"
	"github.com/ultraviolet-black/cruiser/pkg/server"
	"github.com/ultraviolet-black/cruiser/pkg/state"
)

var (
	awsServiceDiscoveryNamespaces []string
	awsServicePortTagKey          string

	awsServiceDiscoveryXds servicediscovery.Xds

	routerServer server.Server

	routerHandler server.SwapHandler

	routesState state.RoutesState

	routerCmd = &cobra.Command{
		Use:   "router",
		Short: "Router is a proxy server for serverless endpoints",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			root := cmd
			for ; root.HasParent(); root = root.Parent() {
			}
			if err := root.PersistentPreRunE(cmd, args); err != nil {
				return err
			}

			routerHandler = server.NewSwapHandler()

			routerServer = server.NewServer(
				server.WithListenerAddress(listenerAddress),
				server.WithShutdownTimeout(shutdownTimeout),
				server.WithListenerProtocol(listenerProtocol),
				server.WithHTTPHandler(routerHandler),
				server.WithTLSConfig(tlsContext),
			)

			routesState = state.NewRoutesState()

			stateManager = state.NewStateManager(
				state.WithTfstateSource(tfstateSource),
				state.WithPeriodicSyncInterval(periodSyncInterval),
				state.WithManagers(routesState),
			)

			return nil

		},
	}

	routerStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the router server",
		RunE: func(cmd *cobra.Command, args []string) error {

			go stateManager.Start(cmd.Context())

			go func() {

				routerOpts := []server.RouterOption{}

				for _, backendProvider := range backendProviders {
					routerOpts = append(routerOpts, server.WithBackendProvider(backendProvider))
				}

				for {
					select {

					case <-cmd.Context().Done():

						observability.Log.Info("router: context cancelled")
						return

					case err := <-stateManager.ErrorCh():

						observability.Log.Error(err.Error())

						signalCh <- os.Kill
						return

					case routesState := <-routesState.UpdateCh():

						observability.Log.Debug("routes update received")

						routes, err := routesState.GetRoutes()
						if err != nil {
							observability.Log.Error(err.Error())
							signalCh <- os.Kill
							return
						}

						routerConfig := &serverpb.Router{
							Routes: routes,
						}

						router := server.NewRouter(
							append(
								routerOpts,
								server.WithRouterConfig(routerConfig),
							)...,
						)

						router.DoHealthcheck(cmd.Context())

						routerHandler.Swap(router)

					}
				}
			}()

			if err := routerServer.Open(); err != nil {
				return err
			}

			waitClose.Wait()

			return nil

		},
		PostRunE: func(cmd *cobra.Command, args []string) error {

			routerHandler.Close()

			return routerServer.Close()

		},
	}
)

func initRouter() {

	routerCmd.PersistentFlags().StringVar(&awsServicePortTagKey, "aws-service-port-tag-key", "port", "AWS service port tag key")
	routerCmd.PersistentFlags().StringSliceVar(&awsServiceDiscoveryNamespaces, "aws-service-discovery-namespaces", []string{}, "AWS service discovery namespaces")

	viper.BindPFlag("aws_service_discovery_namespaces", rootCmd.PersistentFlags().Lookup("aws-service-discovery-namespaces"))
	viper.BindPFlag("aws_service_port_tag_key", rootCmd.PersistentFlags().Lookup("aws-service-port-tag-key"))

}
