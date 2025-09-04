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
import { useHistory } from 'react-router';

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
import { IntegrationKind } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

export const AwsRolesAnywhereSetup = () => {
  const { subPage } = useParams<{ subPage?: string }>();
  const ctx = useTeleport();
  const history = useHistory();
  const integrationsAccess = ctx.storeUser.getIntegrationsAccess();
  const canEnroll = integrationsAccess.create;

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
        <Switch>
          <Route
            exact
            key={IntegrationLevel.Access}
            path={cfg.getIntegrationEnrollRoute(
              IntegrationKind.AwsRa,
              IntegrationLevel.Access
            )}
            component={Access}
          />
          <Route
            exact
            key={IntegrationLevel.Next}
            path={cfg.getIntegrationEnrollRoute(
              IntegrationKind.AwsRa,
              IntegrationLevel.Next
            )}
            component={(props: {
              location: { state: { integrationName: string } };
            }) => {
              return (
                <Finished
                  title="AWS IAM Roles Anywhere Profiles Successfully Imported"
                  resourceText="AWS IAM Roles Anywhere Profiles will be imported soon and available in the Resources page."
                  primaryButtonText="Go To Resources"
                  primaryButtonAction={() =>
                    history.push(
                      `${cfg.getUnifiedResourcesRoute(cfg.proxyCluster)}?kinds=app&query=search("arn:aws:rolesanywhere")`,
                      { kind: 'app' }
                    )
                  }
                  secondaryButtonText="View Integration"
                  secondaryButtonAction={() =>
                    history.push(
                      cfg.getIntegrationStatusRoute(
                        IntegrationKind.AwsRa,
                        props.location.state.integrationName
                      )
                    )
                  }
                />
              );
            }}
          />
          <Route>
            <IamIntegration />
          </Route>
        </Switch>
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
