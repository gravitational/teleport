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
import { IntegrationsSplash } from './IntegrationsSplash';

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
          <IntegrationList list={items} onDelete={onStartDelete} />
        ) : (
          <IntegrationsSplash />
        ))}
      {operation.type === 'delete' && (
        <PluginDelete
          onClose={onCancelDelete}
          onDelete={() => onDelete(operation.plugin)}
        />
      )}
    </FeatureBox>
  );
}
