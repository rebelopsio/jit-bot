package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rebelopsio/jit-bot/internal/config"
	"github.com/rebelopsio/jit-bot/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the JIT access server",
	Long:  `Start the JIT access server to handle Slack requests and manage AWS EKS access.`,
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("port", "8080", "Server port")
	serveCmd.Flags().String("slack-token", "", "Slack bot token")
	serveCmd.Flags().String("slack-signing-secret", "", "Slack signing secret")
	serveCmd.Flags().String("aws-region", "us-east-1", "AWS region")
	serveCmd.Flags().StringSlice("aws-account-ids", []string{}, "AWS account IDs")
	serveCmd.Flags().String("saml-provider-arn", "", "SAML provider ARN")
	serveCmd.Flags().String("eks-cluster-prefix", "", "EKS cluster name prefix")
	serveCmd.Flags().Int("max-access-duration", 3600, "Maximum access duration in seconds")
	serveCmd.Flags().Bool("approval-required", true, "Require approval for access requests")

	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("slack.token", serveCmd.Flags().Lookup("slack-token"))
	viper.BindPFlag("slack.signingSecret", serveCmd.Flags().Lookup("slack-signing-secret"))
	viper.BindPFlag("aws.region", serveCmd.Flags().Lookup("aws-region"))
	viper.BindPFlag("aws.accountIds", serveCmd.Flags().Lookup("aws-account-ids"))
	viper.BindPFlag("aws.samlProviderArn", serveCmd.Flags().Lookup("saml-provider-arn"))
	viper.BindPFlag("aws.eksClusterPrefix", serveCmd.Flags().Lookup("eks-cluster-prefix"))
	viper.BindPFlag("access.maxDuration", serveCmd.Flags().Lookup("max-access-duration"))
	viper.BindPFlag("access.approvalRequired", serveCmd.Flags().Lookup("approval-required"))
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadFromViper()
	if err != nil {
		return err
	}

	srv, err := server.New(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	log.Printf("Starting JIT server on port %s", cfg.Port())
	return srv.Run(ctx)
}
