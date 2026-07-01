package main

import (
	"fmt"

	"github.com/spf13/cobra"

	tmig "github.com/gravitational/teleport/lib/tmig"
	"github.com/gravitational/teleport/lib/tmig/config"
)

func inventoryCmd() *cobra.Command {
	var (
		configPath  string
		format      string
		printConfig bool
	)
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Enumerate agents from SOURCE and resolve mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if printConfig {
				out, err := cfg.EffectiveYAML()
				if err != nil {
					return err
				}
				fmt.Print(string(out))
				return nil
			}
			return tmig.RunInventory(cmd.Context(), cfg, format)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to run.yaml (required)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&printConfig, "print-config", false, "Dump effective config and exit")
	cmd.MarkFlagRequired("config")
	return cmd
}
