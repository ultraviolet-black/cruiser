package cmd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	"github.com/ultraviolet-black/cruiser/pkg/providers/aws"
	"github.com/ultraviolet-black/cruiser/pkg/providers/aws/s3"
	"github.com/ultraviolet-black/cruiser/pkg/server"
	"github.com/ultraviolet-black/cruiser/pkg/state"
	"github.com/ultraviolet-black/cruiser/pkg/tls"
)

var (
	cfgFile string

	signalCh  = make(chan os.Signal)
	waitClose = new(sync.WaitGroup)

	certFile              string
	keyFile               string
	enableTls             bool
	tlsInsecureSkipVerify bool

	tlsContext tls.TlsContext

	listenerAddress string
	shutdownTimeout time.Duration

	periodSyncInterval time.Duration

	tfstateSourceSelector string

	tfstateSource state.TfstateSource

	backendProviders = []server.BackendProvider{}

	dynamodbEndpoint string
	awsProvider      aws.Provider
	awsTfstateBucket string
	awsS3AssumeRole  string

	healthCheckInterval    time.Duration
	healthCheckParallelism = 4

	listenerProtocol server.ListenerProtocol = server.H2C

	stateManager state.StateManager

	rootCmd = &cobra.Command{
		Use:   "cruiser",
		Short: "A small control/data plane for modern cloud native applications",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			if enableTls {

				listenerProtocol = server.HTTP2

				if len(certFile) == 0 || len(keyFile) == 0 {
					tlsCtx, err := tls.FromFile(
						tls.FromFileWithCertificate(certFile, keyFile),
						tls.FromFileWithInsecureSkipVerify(tlsInsecureSkipVerify),
					)

					if err != nil {
						return err
					}

					tlsContext = tlsCtx
				} else {
					tlsCtx, err := tls.FromMemory()

					if err != nil {
						return err
					}

					tlsContext = tlsCtx
				}

			}

			awsProvider = aws.NewProvider(
				aws.WithDynamoDBEndpoint(dynamodbEndpoint),
				aws.WithHealthCheckInterval(healthCheckInterval),
				aws.WithHealthCheckParallelism(healthCheckParallelism),
			)

			backendProviders = append(backendProviders, awsProvider)

			switch tfstateSourceSelector {

			case "aws-s3":

				if len(awsTfstateBucket) == 0 {
					return ErrEmptyAwsTfstateBucket
				}

				s3ClientFactory := func() *awss3.Client {
					return awsProvider.GetS3Client()
				}

				if len(awsS3AssumeRole) > 0 {
					s3ClientFactory = awsProvider.GetS3ClientWithRole(awsS3AssumeRole)
				}

				tfstateSource = s3.NewTfstateSource(
					s3.WithS3ClientFactory(s3ClientFactory),
					s3.WithTfstateBucket(awsTfstateBucket),
				)

			case "":
				observability.Log.Warn("No tfstate source selected, tfstate source will be disabled")

			default:
				return ErrInvalidTfstateSourceSelector

			}

			return nil
		},
	}
)

func Execute() error {

	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		waitClose.Add(1)
		defer waitClose.Done()
		select {
		case <-signalCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	return rootCmd.ExecuteContext(ctx)

}

func init() {
	cobra.OnInitialize(initConfig)

	signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cruiser.toml)")
	rootCmd.PersistentFlags().BoolVar(&enableTls, "enable-tls", true, "enable TLS")
	rootCmd.PersistentFlags().StringVar(&certFile, "tls-certificate", "", "TLS certificate file")
	rootCmd.PersistentFlags().StringVar(&keyFile, "tls-private-key", "", "TLS private key file")
	rootCmd.PersistentFlags().BoolVar(&tlsInsecureSkipVerify, "tls-insecure-skip-verify", false, "TLS insecure skip verify")
	rootCmd.PersistentFlags().StringVar(&listenerAddress, "listener-address", "0.0.0.0:4880", "listener address")
	rootCmd.PersistentFlags().DurationVar(&shutdownTimeout, "shutdown-timeout", 20*time.Second, "shutdown timeout")
	rootCmd.PersistentFlags().DurationVar(&periodSyncInterval, "period-sync-interval", 5*time.Second, "period sync interval")
	rootCmd.PersistentFlags().StringVar(&tfstateSourceSelector, "tfstate-source", "", "tfstate source, valid values: aws-s3")
	rootCmd.PersistentFlags().StringVar(&dynamodbEndpoint, "dynamodb-endpoint", "", "DynamoDB endpoint")
	rootCmd.PersistentFlags().StringVar(&awsTfstateBucket, "aws-tfstate-bucket", "", "AWS tfstate bucket")
	rootCmd.PersistentFlags().DurationVar(&healthCheckInterval, "health-check-interval", 0, "health check interval (0 to disable)")
	rootCmd.PersistentFlags().IntVar(&healthCheckParallelism, "health-check-parallelism", 4, "health check parallelism")
	rootCmd.PersistentFlags().StringVar(&awsS3AssumeRole, "aws-s3-assume-role", "", "AWS S3 assume role")

	viper.BindPFlag("enable_tls", rootCmd.PersistentFlags().Lookup("enable-tls"))
	viper.BindPFlag("tls_certificate", rootCmd.PersistentFlags().Lookup("tls-certificate"))
	viper.BindPFlag("tls_private_key", rootCmd.PersistentFlags().Lookup("tls-private-key"))
	viper.BindPFlag("tls_insecure_skip_verify", rootCmd.PersistentFlags().Lookup("tls-insecure-skip-verify"))
	viper.BindPFlag("listener_address", rootCmd.PersistentFlags().Lookup("listener-address"))
	viper.BindPFlag("shutdown_timeout", rootCmd.PersistentFlags().Lookup("shutdown-timeout"))
	viper.BindPFlag("period_sync_interval", rootCmd.PersistentFlags().Lookup("period-sync-interval"))
	viper.BindPFlag("tfstate_source", rootCmd.PersistentFlags().Lookup("tfstate-source"))
	viper.BindPFlag("dynamodb_endpoint", rootCmd.PersistentFlags().Lookup("dynamodb-endpoint"))
	viper.BindPFlag("aws_tfstate_bucket", rootCmd.PersistentFlags().Lookup("aws-tfstate-bucket"))
	viper.BindPFlag("health_check_interval", rootCmd.PersistentFlags().Lookup("health-check-interval"))
	viper.BindPFlag("health_check_parallelism", rootCmd.PersistentFlags().Lookup("health-check-parallelism"))
	viper.BindPFlag("aws_s3_assume_role", rootCmd.PersistentFlags().Lookup("aws-s3-assume-role"))

	initRouter()

	routerCmd.AddCommand(routerStartCmd)
	rootCmd.AddCommand(routerCmd)
	xdsCmd.AddCommand(xdsStartCmd)
	rootCmd.AddCommand(xdsCmd)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("toml")
		viper.SetConfigName(".cruiser")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		observability.Log.Info("Using config file:", viper.ConfigFileUsed())
	}
}
