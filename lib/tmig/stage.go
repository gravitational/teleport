// Package tmig implements the Teleport scope migration tool.
package tmig

import (
	"context"

	"github.com/gravitational/teleport/lib/tmig/config"
)

// State is a placeholder for the run state that will be implemented in
// lib/tmig/runstate. It tracks per-agent progress across pipeline stages.
// TODO(tmig): Replace with *runstate.State once Task 7 lands.
type State struct{}

// Stage defines the contract for a tmig pipeline stage.
// Each stage (inventory, preflight, migrate, verify) implements this interface.
type Stage interface {
	// Name returns the human-readable stage name (e.g. "inventory", "preflight").
	Name() string
	// Run executes the stage from scratch.
	Run(ctx context.Context, cfg *config.Config, state *State) error
	// Resume continues a previously interrupted stage run.
	Resume(ctx context.Context, cfg *config.Config, state *State) error
	// Status returns the current progress of this stage.
	Status(state *State) StageStatus
}

// StageStatus reports current progress of a stage.
type StageStatus struct {
	Total     int
	Completed int
	Pending   int
	Errors    int
}
