import React from 'react';
import { Indicator, Box, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { IntegrationList } from '@gravitational/teleport/src/Integrations';
import { IntegrationsAddButton } from 'teleport/Integrations/IntegrationsAddButton';

import { useIntegrations, State } from './useIntegrations';
import { PluginDelete } from './PluginDelete';

export default function Container() {
  const state = useIntegrations();
  return <Integrations {...state} />;
}

export function Integrations(props: State) {
  const {
    attempt,
    items,
    onCancelDelete,
    onDelete,
    onStartDelete,
    operation,
    warning,
    canCreateIntegrations,
  } = props;

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Integrations</FeatureHeaderTitle>
        <IntegrationsAddButton canCreate={canCreateIntegrations} />
      </FeatureHeader>
      {warning && <Alert kind="warning" children={warning} />}
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        /* TODO(justinas): redirect to "new integration" when 'items' empty*/
        <IntegrationList list={items} onDelete={onStartDelete} />
      )}
      {operation.type === 'delete' && (
        <PluginDelete
          onClose={onCancelDelete}
          onDelete={() => onDelete(operation.plugin)}
        />
      )}
    </FeatureBox>
  );
}
