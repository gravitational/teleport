/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
import cfg from 'teleport/config';

import { IntegrationsAddButton } from './IntegrationsAddButton';
import { IntegrationList } from './IntegrationList';

import type { Integration } from 'teleport/services/integrations';

export function Integrations() {
  const [items, setItems] = useState<Integration[]>([]);
  const { attempt, run } = useAttempt('processing');

  const ctx = useTeleport();
  const canCreateIntegrations = ctx.storeUser.getIntegrationsAccess().create;

  useEffect(() => {
    run(() =>
      integrationService.fetchIntegrations(cfg.proxyCluster).then(setItems)
    );
  }, []);

  return (
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
        /* TODO(lisa): deletion is stubbed until backend is implemented*/
        <IntegrationList list={items} onDelete={null} />
      )}
    </FeatureBox>
  );
}
