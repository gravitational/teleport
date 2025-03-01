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

import { useEffect, useState } from 'react';

import { Alert, Box, Indicator } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import {
  integrationService,
  type Integration,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { IntegrationList } from './IntegrationList';
import { IntegrationsAddButton } from './IntegrationsAddButton';
import { IntegrationOperations, useIntegrationOperation } from './Operations';
import type { EditableIntegrationFields } from './Operations/useIntegrationOperation';

/**
 * In the web UI, "integrations" can refer to both backend resource
 * type "integration" or "plugin".
 *
 * This open source Integrations component only supports resource
 * type "integration", while its enterprise equivalant component
 * supports both types.
 */
export function Integrations() {
  const integrationOps = useIntegrationOperation();
  const [items, setItems] = useState<Integration[]>([]);
  const { attempt, run } = useAttempt('processing');

  const ctx = useTeleport();

  useEffect(() => {
    // TODO(lisa): handle paginating as a follow up polish.
    // Default fetch is 1k of integrations, which is plenty for beginning.
    run(() =>
      integrationService.fetchIntegrations().then(resp => setItems(resp.items))
    );
  }, []);

  function removeIntegration() {
    return integrationOps.remove().then(() => {
      const updatedItems = items.filter(
        i => i.name !== integrationOps.item.name
      );
      setItems(updatedItems);
      integrationOps.clear();
    });
  }

  function editIntegration(req: EditableIntegrationFields) {
    return integrationOps.edit(req).then(updatedIntegration => {
      const updatedItems = items.map(item => {
        if (item.name == integrationOps.item.name) {
          return updatedIntegration;
        }
        return item;
      });
      setItems(updatedItems);
      integrationOps.clear();
    });
  }

  return (
    <>
      <FeatureBox>
        <FeatureHeader justifyContent="space-between">
          <FeatureHeaderTitle>Integrations</FeatureHeaderTitle>
          <IntegrationsAddButton
            requiredPermissions={[
              {
                value: ctx.storeUser.getIntegrationsAccess().create,
                label: 'integration.create',
              },
            ]}
          />
        </FeatureHeader>
        {attempt.status === 'failed' && <Alert>{attempt.statusText}</Alert>}
        {attempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {attempt.status === 'success' && (
          <IntegrationList
            list={items}
            integrationOps={{
              onDeleteIntegration: integrationOps.onRemove,
              onEditIntegration: integrationOps.onEdit,
            }}
          />
        )}
      </FeatureBox>
      <IntegrationOperations
        operation={integrationOps.type}
        integration={integrationOps.item}
        close={integrationOps.clear}
        remove={removeIntegration}
        edit={editIntegration}
      />
    </>
  );
}
