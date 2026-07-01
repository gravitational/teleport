package main

import (
	"fmt"

	"github.com/spf13/cobra"

	tmig "github.com/gravitational/teleport/lib/tmig"
	"github.com/gravitational/teleport/lib/tmig/config"
)

func preflightCmd() *cobra.Command {
	var (
		configPath  string
		outputDir   string
		format      string
		printConfig bool
		resume      bool
	)
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Run cross-cluster checks and produce the readiness report",
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
			return tmig.RunPreflight(cmd.Context(), cfg, outputDir, format, resume)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to run.yaml (required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for report files")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&printConfig, "print-config", false, "Dump effective config and exit")
	cmd.Flags().BoolVar(&resume, "resume", false, "Merge with prior run state")
	cmd.MarkFlagRequired("config")
	return cmd
}
