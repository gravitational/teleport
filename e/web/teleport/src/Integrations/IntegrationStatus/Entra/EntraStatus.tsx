import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';

import { Alert, Box, ButtonWarning, Flex, Text } from 'design';
import { Trash } from 'design/Icon';

import { EditGroupsImport } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Entra/GroupsImport';
import { SettingsType } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Entra/types';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import { entraPluginUpdate } from 'e-teleport/services/plugins/types';
import { Redirect, Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import {
  Plugin,
  PluginEntraIdSpec,
  PluginEntraIDStatusDetails,
  type Filters,
} from 'teleport/services/integrations';

import { AccessGraphSyncDetails } from './AccessGraphSync';
import { DirectorySyncDetails } from './DirectorySync';
import { GraphApiDetails } from './GraphApi';
import { SsoDetails } from './Sso';

/**
 * EntraStatusRoutes creates Entra ID plugin
 * status page routes.
 */
export function EntraStatusRoutes({
  plugin,
  deletePlugin,
}: {
  plugin: Plugin<PluginEntraIdSpec, PluginEntraIDStatusDetails>;
  deletePlugin(): void;
}) {
  return (
    <Switch>
      <Route
        exact
        key={SettingsType.GroupImport}
        path={cfg.getIntegrationStatusRoute(
          'entra-id',
          plugin.name,
          SettingsType.GroupImport
        )}
      >
        <GroupsImport existingPlugin={plugin} />
      </Route>
      <Route path={cfg.getIntegrationStatusRoute('entra-id', plugin.name)}>
        <StatusDetails onDelete={deletePlugin} plugin={plugin} />
      </Route>
    </Switch>
  );
}

function GroupsImport({
  existingPlugin,
}: {
  existingPlugin: Plugin<PluginEntraIdSpec, PluginEntraIDStatusDetails>;
}) {
  const queryClient = useQueryClient();

  const updatePlugin = useMutation({
    mutationFn: (req: entraPluginUpdate) =>
      pluginsService
        .updatePlugin({
          plugin: 'entra-id',
          entra: { ...req },
        })
        .catch(err => {
          throw err;
        }),
    onSuccess: data =>
      queryClient.setQueryData(
        createFetchPluginQueryKey(existingPlugin.name),
        data
      ),
  });

  const memoizedUpdatePlugin = useCallback(
    (req: entraPluginUpdate) => {
      if (updatePlugin.isPending) {
        return;
      }

      updatePlugin.mutate(req);
    },
    [updatePlugin]
  );

  if (updatePlugin.isSuccess) {
    return (
      <Redirect
        to={cfg.getIntegrationStatusRoute('entra-id', existingPlugin.name)}
      />
    );
  }

  function onSave(
    filters: Filters,
    owners: string[],
    accessListOwnersSource: string
  ) {
    const req: entraPluginUpdate = {
      name: existingPlugin.name,
      defaultOwners: owners,
      groupFilters: filters,
      accessListOwnersSource,
    };
    memoizedUpdatePlugin(req);
  }

  return (
    <>
      {updatePlugin.isError && (
        <Box mt={2}>
          <Alert>{updatePlugin.error.message}</Alert>
        </Box>
      )}
      <EditGroupsImport
        plugin={existingPlugin}
        onSave={onSave}
        disabled={updatePlugin.isPending}
      />
    </>
  );
}

/**
 * StatusDetails displays summary of Entra ID plugin
 * configuration and service status.
 */
export function StatusDetails({
  plugin,
  onDelete,
}: {
  plugin: Plugin<PluginEntraIdSpec, PluginEntraIDStatusDetails>;
  onDelete: () => void;
}) {
  return (
    <Box>
      <Flex
        flexDirection={'row'}
        gap={3}
        mb={3}
        css={`
          @media screen and (max-width: ${p => p.theme.breakpoints.tablet}) {
            gap: ${p => p.theme.space[4]}px;
            margin-bottom: ${p => p.theme.space[4]}px;
            flex-wrap: wrap;
          }
        `}
      >
        <SsoDetails connectorName={plugin.spec.ssoConnectorId} />
        <AccessGraphSyncDetails syncEnabled={plugin.spec.accessGraphEnabled} />
        <GraphApiDetails
          tenantId={plugin.spec.tenantId}
          entraAppId={plugin.spec.entraAppId}
          credentialSource={plugin.spec.credentialSource}
        />
      </Flex>
      <DirectorySyncDetails
        name={plugin.name}
        spec={plugin.spec}
        status={plugin.status}
      />

      <ButtonWarning size="large" onClick={onDelete} mt={4}>
        <Trash mr={2} />
        <Text>Delete Plugin</Text>
      </ButtonWarning>
    </Box>
  );
}
