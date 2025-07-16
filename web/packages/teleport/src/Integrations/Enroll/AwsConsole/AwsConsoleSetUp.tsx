/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { useMemo } from 'react';

import { Alert, Box, Flex } from 'design';

import { EmptyState } from 'teleport/Bots/List/EmptyState/EmptyState';
import { FeatureBox } from 'teleport/components/Layout';
import { Route, Switch, useParams } from 'teleport/components/Router';
import { addIndexToViews } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import cfg from 'teleport/config';
import { Finished } from 'teleport/Discover/Shared';
import { IntegrationIcon } from 'teleport/Integrations/Enroll';
import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { IamIntegration } from 'teleport/Integrations/Enroll/AwsConsole/IamIntegration/IamIntegration';
import { AwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import useTeleport from 'teleport/useTeleport';

export const AwsConsoleSetup = () => {
  const { subPage } = useParams<{ subPage?: string }>();
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const canEnroll = flags.enrollIntegrations;

  const navigationViews = useMemo(
    () =>
      addIndexToViews(
        Object.values(integrationLevels).map(l => ({
          title: l.shortName,
          component: null,
        }))
      ),
    []
  );

  // todo mberg is this level enough?
  if (!canEnroll) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to enroll integrations. Missing role
          permissions: <code>integrations.create</code>
        </Alert>
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <>
      <Box my={4}>
        <Navigation
          currentStep={(integrationLevels[subPage]?.level ?? 0) - 1}
          views={navigationViews}
          startWithIcon={{
            title: 'AWS CLI/Console Access',
            component: <IntegrationIcon size={16} name="aws" />,
          }}
        />
      </Box>
      <Flex flexDirection="column">
        <AwsOidcStatusProvider>
          <Switch>
            <Route
              exact
              key={IntegrationLevel.Access}
              path={`${cfg.routes.integrationEnrollChild}/${IntegrationLevel.Access}`}
              component={Access}
            />
            <Route
              exact
              key={IntegrationLevel.Next}
              path={`${cfg.routes.integrationEnrollChild}/${IntegrationLevel.Next}`}
              component={
                <Finished
                  title="AWS IAM Roles Successfully Imported"
                  resourceText="AWS IAM Roles are successfully imported and will be available on the Resources Page within the next 30 seconds."
                  redirect="" // todo mberg
                />
              }
            />
            <Route>
              <IamIntegration />
            </Route>
          </Switch>
        </AwsOidcStatusProvider>
      </Flex>
    </>
  );
};

enum IntegrationLevel {
  Integration = 'integration',
  Access = 'access',
  Next = 'next',
}

export const integrationLevels = {
  [IntegrationLevel.Integration]: {
    level: 1,
    name: 'Create IAM Roles Anywhere Integration',
    shortName: 'Create IAM Roles Anywhere Integration',
    completeCopy: {},
    bullets: [],
  },
  [IntegrationLevel.Access]: {
    level: 2,
    name: 'Configure Access',
    shortName: 'Configure Access',
    completeCopy: {},
    bullets: [],
  },
  [IntegrationLevel.Next]: {
    level: 3,
    name: 'Next Steps',
    shortName: 'Next Steps',
    bullets: [],
  },
} as const;
