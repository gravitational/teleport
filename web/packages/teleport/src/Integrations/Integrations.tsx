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

import React, { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Indicator, Box, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';
import { integrationService } from 'teleport/services/integrations';

import { IntegrationsAddButton } from './IntegrationsAddButton';
import { IntegrationList } from './IntegrationList';
import { useIntegrationOperation, IntegrationOperations } from './Operations';

import type { Integration } from 'teleport/services/integrations';
import type { EditableIntegrationFields } from './Operations/useIntegrationOperation';

export function Integrations() {
  const integrationOps = useIntegrationOperation();
  const [items, setItems] = useState<Integration[]>([]);
  const { attempt, run } = useAttempt('processing');

  const ctx = useTeleport();
  const canCreateIntegrations = ctx.storeUser.getIntegrationsAccess().create;

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
        <FeatureHeader>
          <FeatureHeaderTitle>Integrations</FeatureHeaderTitle>
          <IntegrationsAddButton canCreate={canCreateIntegrations} />
        </FeatureHeader>
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
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
        integration={integrationOps.item as Integration}
        close={integrationOps.clear}
        remove={removeIntegration}
        edit={editIntegration}
      />
    </>
  );
}
