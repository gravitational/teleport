package main

import "github.com/spf13/cobra"

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tmig",
		Short: "Teleport scope migration tool",
		Long:  "tmig re-homes running Teleport agents from one cluster (or scope) into another.",
	}
	cmd.AddCommand(inventoryCmd())
	cmd.AddCommand(preflightCmd())
	return cmd
}
