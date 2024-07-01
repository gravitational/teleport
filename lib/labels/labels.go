/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package labels provides a way to get dynamic labels. Used by SSH, App,
// and Kubernetes servers.
package labels

import (
	"cmp"
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// DynamicConfig is the configuration for dynamic labels.
type DynamicConfig struct {
	// Labels is the list of dynamic labels to update.
	Labels services.CommandLabels

	// Log is a component logger.
	Log *slog.Logger
}

// CheckAndSetDefaults makes sure valid values were passed in to create
// dynamic labels.
func (c *DynamicConfig) CheckAndSetDefaults() error {
	c.Log = cmp.Or(c.Log, slog.With(teleport.ComponentKey, "dynamiclabels"))

	// Loop over all labels and make sure the key name is valid and the interval
	// is valid as well. If the interval is not valid, update the value.
	labels := c.Labels.Clone()
	for name, label := range labels {
		if len(label.GetCommand()) == 0 {
			return trace.BadParameter("command missing")

		}
		if !types.IsValidLabelKey(name) {
			return trace.BadParameter("invalid label key: %q", name)
		}

		if label.GetPeriod() < time.Second {
			label.SetPeriod(time.Second)
			labels[name] = label
			c.Log.WarnContext(context.Background(), "Label period cannot be less than 1 second. Defaulting to 1 second.", "label", name)
		}
	}

	c.Labels = labels
	return nil
}

// Dynamic allows defining a set of labels whose output is the result
// of some command execution. Dynamic labels can be configured to update
// periodically to provide updated information.
type Dynamic struct {
	mu sync.Mutex
	c  *DynamicConfig

	closeContext context.Context
	closeFunc    context.CancelFunc
}

// NewDynamic returns new Dynamic that can be configured to run
// asynchronously in a loop or synchronously.
func NewDynamic(ctx context.Context, config *DynamicConfig) (*Dynamic, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeFunc := context.WithCancel(ctx)

	return &Dynamic{
		c:            config,
		closeContext: closeContext,
		closeFunc:    closeFunc,
	}, nil
}

// Get returns the list of updated dynamic labels.
func (l *Dynamic) Get() map[string]types.CommandLabel {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make(map[string]types.CommandLabel, len(l.c.Labels))
	for name, label := range l.c.Labels {
		out[name] = label.Clone()
	}

	return out
}

// Sync will block and synchronously update dynamic labels. Used in tests.
func (l *Dynamic) Sync() {
	for name, label := range l.Get() {
		l.updateLabel(name, label)
	}
}

// Start will start a loop that continually keeps dynamic labels updated.
func (l *Dynamic) Start() {
	for name, label := range l.Get() {
		go l.periodicUpdateLabel(name, label)
	}
}

// Close will free up all resources and stop the keeping dynamic labels updated.
func (l *Dynamic) Close() {
	l.closeFunc()
}

// periodicUpdateLabel ticks at the update period defined for each label and
// updates its value.
func (l *Dynamic) periodicUpdateLabel(name string, label types.CommandLabel) {
	ticker := time.NewTicker(label.GetPeriod())
	defer ticker.Stop()

	for {
		l.updateLabel(name, label.Clone())
		select {
		case <-ticker.C:
		case <-l.closeContext.Done():
			return
		}
	}
}

// updateLabel will run a command, then update the value of a label.
func (l *Dynamic) updateLabel(name string, label types.CommandLabel) {
	out, err := exec.Command(label.GetCommand()[0], label.GetCommand()[1:]...).Output()
	if err != nil {
		l.c.Log.ErrorContext(context.Background(), "Failed to run command and update label", "error", err)
		label.SetResult(err.Error() + " output: " + string(out))
	} else {
		label.SetResult(strings.TrimSpace(string(out)))
	}

	// Perform the actual label update under a lock.
	l.setLabel(name, label)
}

// setLabel updates the value of a particular label under a lock.
func (l *Dynamic) setLabel(name string, value types.CommandLabel) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.c.Labels[name] = value
}

// Importer is an interface for labels imported from an external source,
// such as a cloud provider.
type Importer interface {
	// Get returns the current labels.
	Get() map[string]string
	// Apply adds the current labels to the provided resource's static labels.
	Apply(r types.ResourceWithLabels)
	// Sync blocks and synchronously updates the labels.
	Sync(context.Context) error
	// Start starts a loop that continually keeps the labels updated.
	Start(context.Context)
}
