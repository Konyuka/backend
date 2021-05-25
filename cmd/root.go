package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "SmartDial",
	Aliases: []string{"s"},
	Short:   "Backend Service for SmartDial App",
	Long:    `SmartDial is a Backend Service for the interacting with Asterisk call management system.`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}
