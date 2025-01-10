// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msteams

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib/logger"
)

func Uninstall(ctx context.Context, configPath string) error {
	b, c, err := loadConfig(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkApp(ctx, b); err != nil {
		return trace.Wrap(err)
	}

	log := logger.Get(ctx)

	var errs []error
	for _, recipient := range c.Recipients.GetAllRawRecipients() {
		_, isChannel := b.checkChannelURL(recipient)
		if !isChannel {
			errs = append(errs, b.UninstallAppForUser(ctx, recipient))
		}
	}

	if trace.NewAggregate(errs...) != nil {
		log.ErrorContext(ctx, "Encountered error(s) when uninstalling the Teams App", "error", err)
		return err
	}
	log.InfoContext(ctx, "Successfully uninstalled app for all recipients")
	return nil
}
