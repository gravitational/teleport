import { useCallback, useEffect, useState } from 'react';
import { useHistory } from 'react-router';

import { Alert, Box, Flex, H3, Indicator, Text } from 'design';
import { H2, P } from 'design/Text/Text';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import { CtaConnectors } from 'teleport/AuthConnectors/ConnectorList';
import DeleteConnectorDialog from 'teleport/AuthConnectors/DeleteConnectorDialog';
import {
  DesktopDescription,
  MobileDescription,
  ResponsiveAddButton,
  ResponsiveFeatureHeader,
} from 'teleport/AuthConnectors/styles/AuthConnectors.styles';
import { FeatureBox, FeatureHeaderTitle } from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import useResources from 'teleport/components/useResources';
import { KindAuthConnectors, Resource } from 'teleport/services/resources';

import {
  AddNewConnectorPage,
  AddNewConnectorsList,
} from './AddNewConnectorList/AddNewConnectorList';
import { AuthConnectorEditor } from './AuthConnectorEditor';
import ConnectorList from './ConnectorList';
import templates from './templates';

export const description =
  'Auth connectors allow Teleport to authenticate users via an external identity source such as Okta, Microsoft Entra ID, GitHub, etc. This authentication method is commonly known as single sign-on (SSO).';

export default function AuthConnectorsContainer() {
  return (
    <Switch>
      <Route
        key="auth-connector-edit"
        path={cfg.oss.routes.ssoConnector.edit}
        render={() => <AuthConnectorEditor />}
      />
      <Route
        key="auth-connector-create"
        path={cfg.oss.routes.ssoConnector.create}
        exact
        render={() => <AuthConnectorEditor isNew={true} />}
      />
      <Route
        key="auth-connector-new"
        exact
        path={cfg.routes.ssoNewConnectorList}
        render={() => <AddNewConnectorPage />}
      />
      <Route
        exact
        key="auth-connector-list"
        path={cfg.oss.routes.sso}
        render={() => <AuthConnectors />}
      />
    </Switch>
  );
}

export function AuthConnectors() {
  const ctx = useTeleportE();
  const [items, setItems] = useState<Resource<KindAuthConnectors>[]>([]);

  const [fetchAttempt, fetchConnectors] = useAsync(
    useCallback(async () => {
      const response = await ctx.resourceService.fetchAuthConnectors();
      setItems(response);
    }, [ctx.resourceService])
  );

  function remove(connector: Resource<KindAuthConnectors>) {
    const { kind, name } = connector;
    return ctx.resourceService
      .deleteConnector(kind, name)
      .then(fetchConnectors);
  }

  useEffect(() => {
    if (fetchAttempt.status !== 'success') {
      fetchConnectors();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const showAuthConnectorsCTA = ctx.lockedFeatures.authConnectors;

  const history = useHistory();
  const isEmpty = items.length === 0;
  const resources = useResources(items, templates);

  return (
    <FeatureBox>
      <ResponsiveFeatureHeader>
        <FeatureHeaderTitle>Auth Connectors</FeatureHeaderTitle>
        <MobileDescription>{description}</MobileDescription>
        {(!showAuthConnectorsCTA || !isEmpty) && (
          <ResponsiveAddButton
            fill="border"
            onClick={() => history.push(cfg.routes.ssoNewConnectorList)}
          >
            Add Auth Connector
          </ResponsiveAddButton>
        )}
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
              <ConnectorList items={items} onDelete={resources.remove} />
            </Box>
            {isEmpty && !showAuthConnectorsCTA && (
              <Box>
                <H2 mb={4}>Enroll a Single Sign-On Connector</H2>
                <AddNewConnectorsList />
              </Box>
            )}
            {showAuthConnectorsCTA && <CtaConnectors />}
          </Flex>
          <DesktopDescription>
            <H3 mb={3}>Auth Connectors</H3>
            <P>{description}</P>
            <P>
              Please{' '}
              <Text
                as="a"
                color="text.main"
                href="https://goteleport.com/docs/admin-guides/access-controls/sso/"
                target="_blank"
              >
                view our documentation
              </Text>{' '}
              for samples of each connector.
            </P>
          </DesktopDescription>
        </Flex>
      )}
      {resources.status === 'removing' && (
        <DeleteConnectorDialog
          name={resources.item.name}
          kind={resources.item.kind}
          onClose={resources.disregard}
          onDelete={() => remove(resources.item)}
        />
      )}
    </FeatureBox>
  );
}
