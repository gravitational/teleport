import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';

import { Alert, Box, Flex, Indicator } from 'design';
import { H2 } from 'design/Text/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import { InfoGuide } from 'teleport/AuthConnectors/AuthConnectors';
import {
  ConnectorList,
  CtaConnectors,
} from 'teleport/AuthConnectors/ConnectorList';
import DeleteConnectorDialog from 'teleport/AuthConnectors/DeleteConnectorDialog';
import {
  ResponsiveAddButton,
  ResponsiveFeatureHeader,
} from 'teleport/AuthConnectors/styles/AuthConnectors.styles';
import { FeatureBox, FeatureHeaderTitle } from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import useResources from 'teleport/components/useResources';
import {
  DefaultAuthConnector,
  KindAuthConnectors,
  Resource,
} from 'teleport/services/resources';

import {
  AddNewConnectorPage,
  AddNewConnectorsList,
} from './AddNewConnectorList/AddNewConnectorList';
import { AuthConnectorEditor } from './AuthConnectorEditor';
import templates from './templates';

export default function AuthConnectorsContainer() {
  return (
    <Switch>
      <Route
        key="auth-connector-edit"
        path={cfg.oss.routes.ssoConnector.edit}
        element={<AuthConnectorEditor />}
      />
      <Route
        key="auth-connector-create"
        path={cfg.oss.routes.ssoConnector.create}
        exact
        element={<AuthConnectorEditor isNew={true} />}
      />
      <Route
        key="auth-connector-new"
        exact
        path={cfg.routes.ssoNewConnectorList}
        element={<AddNewConnectorPage />}
      />
      <Route
        exact
        key="auth-connector-list"
        path={cfg.oss.routes.sso}
        element={<AuthConnectors />}
      />
    </Switch>
  );
}

export function AuthConnectors() {
  const ctx = useTeleportE();
  const [items, setItems] = useState<Resource<KindAuthConnectors>[]>([]);
  const [defaultConnector, setDefaultConnector] =
    useState<DefaultAuthConnector>();

  const [fetchAttempt, fetchConnectors] = useAsync(
    useCallback(async () => {
      return await ctx.resourceService.fetchAuthConnectors().then(res => {
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

  const navigate = useNavigate();
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
        {(!showAuthConnectorsCTA || !isEmpty) && (
          <InfoGuideButton config={{ guide: <InfoGuide /> }}>
            <ResponsiveAddButton
              fill="border"
              onClick={() => navigate(cfg.routes.ssoNewConnectorList)}
            >
              Add Auth Connector
            </ResponsiveAddButton>
          </InfoGuideButton>
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
              {setDefaultAttempt.status === 'error' && (
                <Alert>
                  Failed to set connector as default:{' '}
                  {setDefaultAttempt.statusText}
                </Alert>
              )}
              <ConnectorList
                items={items}
                onDelete={resources.remove}
                defaultConnector={defaultConnector}
                setAsDefault={onUpdateDefaultConnector}
              />
            </Box>
            {isEmpty && !showAuthConnectorsCTA && (
              <Box>
                <H2 mb={4}>Enroll a Single Sign-On Connector</H2>
                <AddNewConnectorsList />
              </Box>
            )}
            {showAuthConnectorsCTA && <CtaConnectors />}
          </Flex>
        </Flex>
      )}
      {resources.status === 'removing' && (
        <DeleteConnectorDialog
          name={resources.item.name}
          kind={resources.item.kind}
          onClose={resources.disregard}
          onDelete={() => remove(resources.item)}
          isDefault={defaultConnector?.name === resources?.item?.name}
          nextDefault={nextDefaultConnector}
        />
      )}
    </FeatureBox>
  );
}
