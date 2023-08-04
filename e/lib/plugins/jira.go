/*
Copyright 2023 Gravitational, Inc.

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

package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/jira"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func jiraInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	jiraSpec := plugin.Spec.GetJira()
	if jiraSpec == nil {
		return nil, trace.BadParameter("field Spec.Jira must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("missing Jira plugin static credentials")
	}

	// JIRA API tokens are essentially alternative passwords for a given user,
	// so we treat jira creds like basic auth.
	jiraUsername, jiraToken := deps.staticCredentials[0].GetBasicAuth()

	cfg := jira.Config{
		Jira: jira.JiraConfig{
			URL:       jiraSpec.ServerUrl,
			APIToken:  jiraToken,
			Project:   jiraSpec.ProjectKey,
			IssueType: jiraSpec.IssueType,
			Username:  jiraUsername,
		},
		Client:         deps.client,
		StatusSink:     deps.statusSink,
		DisableWebhook: true,
	}

	app, err := jira.NewApp(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appCtx := logger.WithLogger(deps.lifetime, deps.log)
	return func() error {
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}
