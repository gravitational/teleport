import React from 'react';
import { Indicator, Box, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { IntegrationList } from '@gravitational/teleport/src/Integrations';
import { IntegrationsAddButton } from 'teleport/Integrations/IntegrationsAddButton';
import { Integration, Plugin } from 'teleport/services/integrations';
import { IntegrationOperations } from 'teleport/Integrations/Operations';

import { useIntegrations, State } from './useIntegrations';
import { PluginDelete } from './PluginDelete';
import { IntegrationsSplash } from './IntegrationsSplash';

export default function Container() {
  const state = useIntegrations();
  return <Integrations {...state} />;
}

export function Integrations(props: State) {
  const {
    attempt,
    items,
    pluginOps,
    integrationOps,
    warning,
    canCreateIntegrations,
  } = props;

  const hasItems = items.length !== 0;

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Integrations</FeatureHeaderTitle>
        {hasItems && (
          <IntegrationsAddButton canCreate={canCreateIntegrations} />
        )}
      </FeatureHeader>
      {warning && <Alert kind="warning" children={warning} />}
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' &&
        (hasItems ? (
          <IntegrationList
            list={items}
            onDeletePlugin={pluginOps.onStartDelete}
            integrationOps={{
              onDeleteIntegration: integrationOps.onRemove,
              onEditIntegration: integrationOps.onEdit,
            }}
          />
        ) : (
          <IntegrationsSplash />
        ))}
      {pluginOps.type === 'delete' && (
        <PluginDelete
          onClose={pluginOps.onCancelDelete}
          onDelete={() => pluginOps.onDelete(pluginOps.item as Plugin)}
        />
      )}
      <IntegrationOperations
        operation={integrationOps.type}
        integration={integrationOps.item as Integration}
        close={integrationOps.clear}
        remove={integrationOps.removeIntegration}
        edit={integrationOps.editIntegration}
      />
    </FeatureBox>
  );
}
