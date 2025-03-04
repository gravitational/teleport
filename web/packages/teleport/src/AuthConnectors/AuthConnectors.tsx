/**
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

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useHistory } from 'react-router';

import { Alert, Box, Flex, Indicator } from 'design';
import { H2 } from 'design/Text/Text';
import { useAsync } from 'shared/hooks/useAsync';

import {
  ResponsiveAddButton,
  ResponsiveFeatureHeader,
} from 'teleport/AuthConnectors/styles/AuthConnectors.styles';
import { FeatureBox, FeatureHeaderTitle } from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import {
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
} from 'teleport/components/SlidingSidePanel/InfoGuideSidePanel';
import useResources from 'teleport/components/useResources';
import cfg from 'teleport/config';
import { DefaultAuthConnector, Resource } from 'teleport/services/resources';
import useTeleport from 'teleport/useTeleport';

import { GitHubConnectorEditor } from './AuthConnectorEditor';
import { ConnectorList } from './ConnectorList';
import { CtaConnectors } from './ConnectorList/CTAConnectors';
import DeleteConnectorDialog from './DeleteConnectorDialog';
import EmptyList from './EmptyList';
import templates from './templates';

export const description =
  'Auth connectors allow Teleport to authenticate users via an external identity source such as Okta, Microsoft Entra ID, GitHub, etc. This authentication method is commonly known as single sign-on (SSO).';

/**
 * AuthConnectorsContainer is the container for the Auth Connectors feature and handles routing to the relevant page based on the URL.
 */
export function AuthConnectorsContainer() {
  return (
    <Switch>
      <Route
        key="auth-connector-edit"
        path={cfg.routes.ssoConnector.edit}
        render={() => <GitHubConnectorEditor />}
      />
      <Route
        key="auth-connector-new"
        path={cfg.routes.ssoConnector.create}
        render={() => <GitHubConnectorEditor isNew={true} />}
      />
      <Route
        key="auth-connector-list"
        path={cfg.routes.sso}
        exact
        render={() => <AuthConnectors />}
      />
    </Switch>
  );
}

/**
 * AuthConnectors is the auth connectors list page.
 */
export function AuthConnectors() {
  const ctx = useTeleport();
  const [items, setItems] = useState<Resource<'github'>[]>([]);
  const [defaultConnector, setDefaultConnector] =
    useState<DefaultAuthConnector>();

  const [fetchAttempt, fetchConnectors] = useAsync(
    useCallback(async () => {
      return await ctx.resourceService.fetchGithubConnectors().then(res => {
        setItems(res.connectors);
        setDefaultConnector(res.defaultConnector);
      });
    }, [ctx.resourceService])
  );

  const [setDefaultAttempt, updateDefaultConnector] = useAsync(
    async (connector: DefaultAuthConnector) =>
      await ctx.resourceService.setDefaultAuthConnector(connector)
  );

  function onUpdateDefaultConnector(connector: DefaultAuthConnector) {
    const originalDefault = defaultConnector;
    setDefaultConnector(connector);
    updateDefaultConnector(connector).catch(err => {
      // Revert back to the original default if the operation failed.
      setDefaultConnector(originalDefault);
      throw err;
    });
  }

  function remove(name: string) {
    return ctx.resourceService
      .deleteGithubConnector(name)
      .then(fetchConnectors);
  }

  useEffect(() => {
    if (fetchAttempt.status !== 'success') {
      fetchConnectors();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const history = useHistory();
  const isEmpty = items.length === 0;
  const resources = useResources(items, templates);

  // Calculate the next default connector.
  const nextDefaultConnector = useMemo(() => {
    // If there is only one (or no) connectors, the fallback will always be "local"
    if (items.length < 2) {
      return 'Local Connector';
    }
    // If the connector being removed is last in the list, the next default will be the second last connector.
    if (items[items.length - 1].name === resources?.item?.name) {
      return items[items.length - 2].name;
    } else {
      // If the connector being removed isn't the last connector, the next default will always be the last connector.
      return items[items.length - 1].name;
    }
  }, [items, resources.item]);

  return (
    <FeatureBox>
      <ResponsiveFeatureHeader>
        <FeatureHeaderTitle>Auth Connectors</FeatureHeaderTitle>
        <ResponsiveAddButton
          fill="border"
          onClick={() =>
            history.push(cfg.getCreateAuthConnectorRoute('github'))
          }
        >
          New GitHub Connector
        </ResponsiveAddButton>
      </ResponsiveFeatureHeader>
      {fetchAttempt.status === 'error' && (
        <Alert children={fetchAttempt.statusText} />
      )}
      {fetchAttempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {fetchAttempt.status === 'success' && (
        <Flex alignItems="start">
          <Flex flexDirection="column" width="100%" gap={5}>
            <Box>
              <H2 mb={4}>Your Connectors</H2>
              {setDefaultAttempt.status === 'error' && (
                <Alert>
                  Failed to set connector as default:{' '}
                  {setDefaultAttempt.statusText}
                </Alert>
              )}
              {isEmpty ? (
                <EmptyList
                  onCreate={() =>
                    history.push(cfg.getCreateAuthConnectorRoute('github'))
                  }
                  isLocalDefault={defaultConnector.type === 'local'}
                />
              ) : (
                <ConnectorList
                  items={items}
                  onDelete={resources.remove}
                  defaultConnector={defaultConnector}
                  setAsDefault={onUpdateDefaultConnector}
                />
              )}
            </Box>
            <CtaConnectors />
          </Flex>
        </Flex>
      )}
      {resources.status === 'removing' && (
        <DeleteConnectorDialog
          name={resources.item.name}
          kind={resources.item.kind}
          onClose={resources.disregard}
          onDelete={() => remove(resources.item.name)}
          isDefault={defaultConnector.name === resources.item.name}
          nextDefault={nextDefaultConnector}
        />
      )}
    </FeatureBox>
  );
}

export const InfoGuide = ({ isGitHub = false }) => (
  <Box>
    <InfoTitle>Auth Connectors</InfoTitle>
    <InfoParagraph>
      Auth connectors allow Teleport to authenticate users via an external
      identity source such as Okta, Microsoft Entra ID, GitHub, etc. This
      authentication method is commonly known as single sign-on (SSO).
    </InfoParagraph>
    <ReferenceLinks
      links={[
        isGitHub
          ? {
              title: 'Configure GitHub connector',
              href: 'https://goteleport.com/docs/admin-guides/access-controls/sso/github-sso/',
            }
          : {
              title: 'Samples of different connectors',
              href: 'https://goteleport.com/docs/admin-guides/access-controls/sso/',
            },
      ]}
    />
  </Box>
);
