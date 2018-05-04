package cmd

import (
	"fmt"
	"os"
	"upload/config"
	"upload/server"

	"github.com/fabbricadigitale/scimd/validation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	validator "gopkg.in/go-playground/validator.v9"
)

var upload = &cobra.Command{
	Use: "upload",
	// TraverseChildren: true,
	Short: "UPLOAD is ...",
	Long:  `...`,
	PreRun: func(cmd *cobra.Command, args []string) {
		// Flags overrides (or merge with) configuration values
		// Thus we re-validate the configuration prior to execute and we collect errors
		if _, err := config.Valid(); err != nil {
			errors, _ := err.(validator.ValidationErrors)
			config.Errors = append(config.Errors, errors...)
		}

		// Printing errors
		if len(config.Errors) > 0 {
			fmt.Fprintln(os.Stderr, validation.Errors(config.Errors))
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Start the server with the current service provider config
		if config.Values.Debug {
			fmt.Println("Running in debug mode")
		}
		server.Run(config.Values.Host, config.Values.Port)
	},
}

func init() {

	upload.PersistentFlags().BoolVar(&config.Values.Debug, "debug", config.Values.Debug, "wheter to enable or not the debug mode")
	/* upload.Flags().StringVarP(&config.Values.Port, "port", "p", config.Values.Port, "port to run the server on")
	upload.Flags().StringVarP(&config.Values.Host, "host", "h", config.Values.Host, "address of the server") */

	// Binding flags to configuration manager
	viper.BindPFlags(upload.Flags())
}

// Execute is starting point for commands
func Execute() {
	if err := upload.Execute(); err != nil {
		os.Exit(1)
	}
}
