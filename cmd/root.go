package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	Version = "v0.3.1"

	configFile string
	logLevel string

	logger *log.Logger
)

func Execute() error {
	cobra.OnInitialize(setupLogger)

	rootCmd := &cobra.Command{
		Use:   "hcscli",
		Short: "A command line tool for HCS related operations",
		Long: `hcscli is a command line tool for various tasks including HCS topic creation / deletion, 
hedera account creation, balance transfer, and etc.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return loadConfig(configFile)
		},
		Version: Version,
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "hedera_env.json", "config file")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "error", "logging level")

	rootCmd.AddCommand(buildAccountCommand())
	rootCmd.AddCommand(buildTopicCommand())
	rootCmd.AddCommand(buildKeyCommand())

	return rootCmd.Execute()
}

func setupLogger() {
	logger = log.New()

	if level, err := log.ParseLevel(logLevel); err == nil {
		logger.SetLevel(level)
	}

	logger.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	logger.SetOutput(os.Stdout)
}
