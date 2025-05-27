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

import {
  ComponentProps,
  PropsWithChildren,
  ReactNode,
  useMemo,
  useState,
} from 'react';
import { useHistory } from 'react-router';
import styled from 'styled-components';

import { Box, Flex, H2 } from 'design';
import { SpaceProps } from 'design/system';

import { Route, Switch, useParams } from 'teleport/components/Router';
import { addIndexToViews } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import cfg from 'teleport/config';
import { IntegrationIcon } from 'teleport/Integrations/Enroll';
import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { IamIntegration } from 'teleport/Integrations/Enroll/AwsConsole/IamIntegration/IamIntegration';
import { AwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import { IntegrationKind } from 'teleport/services/integrations';

//  todo mberg make this generic and reusable in /shared

export const AwsConsoleSetup = () => {
  //  todo mberg; oidc integration not found

  const history = useHistory();
  const { subPage } = useParams<{ subPage?: string }>();

  const [completedSteps, setCompletedSteps] = useState<
    Record<IntegrationLevel, boolean>
  >({} as Record<IntegrationLevel, boolean>);
  const highestCompletedStep = useMemo<IntegrationLevel | undefined>(() => {
    for (const step of [
      IntegrationLevel.Integration,
      IntegrationLevel.Access,
      IntegrationLevel.Next,
    ]) {
      if (completedSteps[step]) {
        return step;
      }
    }
    return undefined;
  }, [completedSteps]);

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

  const onContinue = async (level: IntegrationLevel) => {
    // On mount, fetch any existing Okta plugin
    // loading state
    // todo mberg
  };

  return (
    <>
      <Box my={4}>
        <Navigation
          currentStep={(integrationLevels[subPage]?.level ?? 0) - 1}
          views={navigationViews}
          startWithIcon={{
            title: 'AWS CLI/Console Access',
            component: <PluginIcon type="aws" size={16} />,
          }}
        />
      </Box>
      <Flex flexDirection="column" gap={4}>
        <AwsOidcStatusProvider>
          <Switch>
            <Route
              exact
              key={IntegrationKind.AwsConsole}
              path={`${cfg.routes.integrationEnrollChild}/access`}
              component={Access}
            />
            <Route
              exact
              key={IntegrationLevel.Next}
              path={cfg.getIntegrationEnrollRoute('aws', IntegrationLevel.Next)}
              component={'todo'}
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
    shortName: 'IAM Roles Anywhere',
    completeCopy: {},
    bullets: [],
  },
  [IntegrationLevel.Access]: {
    level: 2,
    name: 'Configure Access',
    shortName: 'Access',
    completeCopy: {},
    bullets: [],
  },
  [IntegrationLevel.Next]: {
    level: 3,
    name: 'Next Steps',
    shortName: 'Next',
    bullets: [],
  },
} as const;

export const getNextIntegrationLevel = (
  current: IntegrationLevel | undefined
) => {
  if (!current) {
    return IntegrationLevel.Integration;
  }
  switch (current) {
    case IntegrationLevel.Integration:
      return IntegrationLevel.Access;
    case IntegrationLevel.Access:
      return IntegrationLevel.Next;
    default:
      return undefined;
  }
};

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared.tsx
const StyledBoxComponent = styled(Flex)`
  position: relative;
  background-color: ${props => props.theme.colors.levels.surface};
  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared.tsx
export const StyledBox = ({
  header,
  children,
  ...props
}: PropsWithChildren<
  {
    header?: string | ReactNode;
  } & ComponentProps<typeof StyledBoxComponent>
>) => (
  <StyledBoxComponent
    p={4}
    gap={3}
    borderRadius={3}
    flexDirection="column"
    maxWidth="800px"
    {...props}
  >
    {typeof header === 'string' ? (
      <H2 mt={-1}>{header}</H2>
    ) : typeof header !== 'undefined' ? (
      <Box mt={-1}>{header}</Box>
    ) : null}
    {children}
  </StyledBoxComponent>
);

//  from e/web/teleport/src/Integrations/IntegrationEnroll/IntegrationPick/PluginIcon.tsx
export function PluginIcon({ size, type, ...props }: Props) {
  return <IntegrationIcon {...props} size={size} name={'aws'} />;
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/IntegrationPick/PluginIcon.tsx
interface Props extends SpaceProps {
  size?: number;
  type: string;
}
