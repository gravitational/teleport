/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bot

import (
	"context"
	"strings"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/env"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/review"

	"github.com/gravitational/trace"
)

// Config contains configuration for the bot.
type Config struct {
	// GitHub is a GitHub client.
	GitHub github.Client

	// Environment holds information about the workflow run event.
	Environment *env.Environment

	// Review is used to get code and docs reviewers.
	Review *review.Assignments
}

// CheckAndSetDefaults checks and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.GitHub == nil {
		return trace.BadParameter("missing parameter GitHub")
	}
	if c.Environment == nil {
		return trace.BadParameter("missing parameter Environment")
	}
	if c.Review == nil {
		return trace.BadParameter("missing parameter Review")
	}

	return nil
}

// Bot performs repository management.
type Bot struct {
	c *Config
}

// New returns a new repository management bot.
func New(c *Config) (*Bot, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Bot{
		c: c,
	}, nil
}

func (b *Bot) parseChanges(ctx context.Context) (bool, bool, error) {
	var docs bool
	var code bool

	files, err := b.c.GitHub.ListFiles(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return false, true, trace.Wrap(err)
	}

	for _, file := range files {
		if hasDocs(file) {
			docs = true
		} else {
			code = true
		}

	}
	return docs, code, nil
}

func hasDocs(filename string) bool {
	return strings.HasPrefix(filename, "docs/") ||
		strings.HasSuffix(filename, ".md") ||
		strings.HasSuffix(filename, ".mdx") ||
		strings.HasPrefix(filename, "rfd/")
}
