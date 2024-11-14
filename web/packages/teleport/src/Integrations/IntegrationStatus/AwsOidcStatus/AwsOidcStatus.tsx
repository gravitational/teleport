import React from 'react';
import { useParams } from 'react-router';

import { Switch, Route } from 'teleport/components/Router';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

import { AwsOidcStatusProvider } from './useAwsOidcStatus';
import { AwsOidcDashboard } from './Dashboard/AwsOidcDashboard';
import { AwsOidcResources } from './Resources/AwsOidcResources';
import { AwsResourceKind } from '../Shared';

export function AwsOidcStatus() {
  const {
    type: integrationType,
    name: integrationName,
    resourceKind,
  } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind?: AwsResourceKind;
  }>();

  console.log('--- here i am??');
  return (
    <AwsOidcStatusProvider>
      <Switch>
        <Route
          key="aws-oidc-resources-list"
          exact
          path={cfg.routes.integrationStatusResources}
          component={AwsOidcResources}
        />
        {/* <Route
          key="aws-oidc-tasks"
          exact
          path={cfg.getIntegrationStatusTasksRoute(integrationType, integrationName)}
          component={AwsOidcTasks}
        /> */}
        <Route
          key="aws-oidc-dashboard"
          exact
          path={cfg.routes.integrationStatus}
          component={AwsOidcDashboard}
        />
      </Switch>
    </AwsOidcStatusProvider>
  );
}
