import React from 'react';
import { Indicator, Box, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { IntegrationList } from 'teleport/Integrations';
import { IntegrationsAddButton } from 'teleport/Integrations/IntegrationsAddButton';
import { Integration, Plugin } from 'teleport/services/integrations';
import { IntegrationOperations } from 'teleport/Integrations/Operations';

import { ExternalAuditStorageOpType } from 'teleport/Integrations/Operations/useIntegrationOperation';

import { useIntegrations, State } from './useIntegrations';
import { PluginDelete } from './PluginDelete';
import { IntegrationsSplash } from './IntegrationsSplash';
import { ExternalAuditStorageDelete } from './ExternalAuditStorageDelete';

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
    requiredCreatePermissions,
    externalAuditStorageOps,
    warning,
    auditStorageAttempt,
  } = props;

  const hasItems = items.length !== 0;

  return (
    <FeatureBox>
      <FeatureHeader justifyContent="space-between">
        <FeatureHeaderTitle>Integrations</FeatureHeaderTitle>
        {hasItems && (
          <IntegrationsAddButton
            requiredPermissions={requiredCreatePermissions}
          />
        )}
      </FeatureHeader>
      {warning && <Alert kind="warning" children={warning} />}
      {auditStorageAttempt.status === 'failed' && (
        <Alert
          children={`Failed removing external audit: ${auditStorageAttempt.statusText}`}
        />
      )}
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
            onDeleteExternalAuditStorage={
              externalAuditStorageOps.onStartDeleteExternalAuditStorage
            }
          />
        ) : (
          <IntegrationsSplash />
        ))}
      {pluginOps.type === 'delete' && (
        <PluginDelete
          onClose={pluginOps.onCancelDelete}
          onDelete={() => pluginOps.onDelete(pluginOps.item as Plugin)}
          pluginKind={(pluginOps.item as Plugin).kind}
        />
      )}
      {externalAuditStorageOps.type === 'delete' && (
        <ExternalAuditStorageDelete
          onClose={externalAuditStorageOps.onCancelDeleteExternalAuditStorage}
          onDelete={() =>
            externalAuditStorageOps.onDeleteExternalAuditStorage()
          }
          opType={
            externalAuditStorageOps.item.name as ExternalAuditStorageOpType
          }
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
