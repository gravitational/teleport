/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package provisioning

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/prompt"
)

// validName is used to ensure that [OperationConfig] and [ActionConfig] names
// start with a letter and only consist of letters, numbers, and hyphen
// characters thereafter.
var validName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]+$`)

// OperationConfig is the configuration for an operation.
type OperationConfig struct {
	// Name is the operation name. Must consist of only letters and hyphens.
	Name string
	// Actions is the list of actions that make up the operation.
	Actions []Action
	// AutoConfirm is whether to skip the operation plan confirmation prompt.
	AutoConfirm bool
	// Output is an [io.Writer] where the operation plan and confirmation prompt
	// are written to.
	// Defaults to [os.Stdout].
	Output io.Writer
}

// checkAndSetDefaults validates the operation config and sets defaults.
func (c *OperationConfig) checkAndSetDefaults() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return trace.BadParameter("missing operation name")
	}
	if !validName.MatchString(c.Name) {
		return trace.BadParameter(
			"operation name %q does not match regex used for validation %q",
			c.Name, validName.String(),
		)
	}
	if len(c.Actions) == 0 {
		return trace.BadParameter("missing operation actions")
	}
	if c.Output == nil {
		c.Output = os.Stdout
	}
	return nil
}

// Action wraps a runnable function to provide a name, summary, and detailed
// explanation of the behavior.
type Action struct {
	// config is an unexported value-type to prevent mutation after it's been
	// validated by the checkAndSetDefaults func.
	config ActionConfig
}

// GetName returns the action's configured name.
func (a *Action) GetName() string {
	return a.config.Name
}

// GetSummary returns the action's configured summary.
func (a *Action) GetSummary() string {
	return a.config.Summary
}

// GetDetails returns the action's configured details.
func (a *Action) GetDetails() string {
	return a.config.Details
}

// Run runs the action.
func (a *Action) Run(ctx context.Context) error {
	return a.config.RunnerFn(ctx)
}

// NewAction creates a new [Action].
func NewAction(config ActionConfig) (*Action, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Action{
		config: config,
	}, nil
}

// ActionConfig is the configuration for an [Action].
type ActionConfig struct {
	// Name is the action name.
	Name string
	// Summary is the action summary in prose.
	Summary string
	// Details is the detailed explanation of the action, explaining exactly
	// what it will do.
	Details string
	// RunnerFn is a function that actually runs the action.
	RunnerFn func(context.Context) error
}

// checkAndSetDefaults validates the action config and sets defaults.
func (c *ActionConfig) checkAndSetDefaults() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Summary = strings.TrimSpace(c.Summary)
	c.Details = strings.TrimSpace(c.Details)
	if c.Name == "" {
		return trace.BadParameter("missing action name")
	}
	if !validName.MatchString(c.Name) {
		return trace.BadParameter(
			"action name %q does not match regex used for validation %q",
			c.Name, validName.String(),
		)
	}
	if c.Summary == "" {
		return trace.BadParameter("missing action summary")
	}
	if c.Details == "" {
		return trace.BadParameter("missing action details")
	}
	if c.RunnerFn == nil {
		return trace.BadParameter("missing action runner")
	}

	return nil
}

// Run writes the operation plan, optionally prompts for user confirmation,
// then executes the operation plan.
func Run(ctx context.Context, config OperationConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := writeOperationPlan(config); err != nil {
		return trace.Wrap(err)
	}

	if !config.AutoConfirm {
		ok, err := prompt.Confirmation(ctx, config.Output, prompt.Stdin(), getPromptQuestion(config))
		if err != nil {
			return trace.Wrap(err)
		}
		if !ok {
			return trace.BadParameter("operation %q canceled", config.Name)
		}
	}

	enumerateSteps := len(config.Actions) > 1
	for i, action := range config.Actions {
		if enumerateSteps {
			slog.InfoContext(ctx, "Running", "step", i+1, "action", action.config.Name)
		} else {
			slog.InfoContext(ctx, "Running", "action", action.config.Name)
		}

		if err := action.Run(ctx); err != nil {
			if enumerateSteps {
				return trace.Wrap(err, "step %d %q failed", i+1, action.config.Name)
			}
			return trace.Wrap(err, "%q failed", action.config.Name)
		}
	}
	slog.InfoContext(ctx, "Success!", "operation", config.Name)
	return nil
}

// writeOperationPlan writes the operational plan to the given [io.Writer] as
// a structured summary of the operation and the actions that compose it.
func writeOperationPlan(config OperationConfig) error {
	data := map[string]any{
		"config":          config,
		"showStepNumbers": len(config.Actions) > 1,
	}
	return trace.Wrap(operationPlanTemplate.Execute(config.Output, data))
}

var operationPlanTemplate = template.Must(template.New("plan").
	Funcs(template.FuncMap{
		// used to enumerate the action steps starting from 1 instead of 0.
		"addOne": func(x int) int { return x + 1 },
	}).
	Parse(`
{{- printf "%q" .config.Name }} will perform the following {{ if .showStepNumbers }}actions{{ else }}action{{ end }}:

{{ $global := . }}
{{- range $index, $action := .config.Actions }}
{{- if $global.showStepNumbers }}{{ addOne $index }}. {{ end -}}{{$action.GetSummary}}.
{{$action.GetName}}: {{$action.GetDetails}}

{{end -}}
`))

func getPromptQuestion(config OperationConfig) string {
	if len(config.Actions) > 1 {
		return fmt.Sprintf("Do you want %q to perform these actions?", config.Name)
	}
	return fmt.Sprintf("Do you want %q to perform this action?", config.Name)
}
