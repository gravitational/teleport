/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { ButtonSecondary, Box, Indicator } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { useAsync } from 'shared/hooks/useAsync';
import { useTheme } from 'styled-components';
import { SamlMeta } from 'teleport/Discover/useDiscover';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import {
  SamlIdpServiceProvider,
  SamlServiceProviderPreset,
} from 'teleport/services/samlidp/types';

import useTeleportE from 'e-teleport/useTeleportE';
import { DiscoverUpdate } from 'e-teleport/Discover';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

export function UpdateSamlApplication({
  resourceSpec,
}: {
  resourceSpec: ResourceSpec;
}) {
  const theme = useTheme();
  const { idpService } = useTeleportE();

  const [open, setOpen] = useState(false);

  const [fetchSamlResourceAttempt, fetchSamlResource] = useAsync(
    async (name: string) => {
      const resp = await idpService.getSamlIdpServiceProvider(name);
      return samlResponseToAgentMeta(resp);
    }
  );

  useEffect(() => {
    if (resourceSpec) {
      setOpen(true);
      fetchSamlResource(resourceSpec.name);
    }
  }, [resourceSpec]);

  function content() {
    switch (fetchSamlResourceAttempt.status) {
      case '':
      case 'processing':
        return (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        );
      case 'error':
        return <ErrorMessage message={fetchSamlResourceAttempt.statusText} />;
      case 'success':
        return (
          <DiscoverUpdate
            resourceSpec={resourceSpec}
            agentMeta={fetchSamlResourceAttempt.data}
          />
        );
    }
  }

  return (
    <Dialog
      dialogCss={() => ({
        height: '100%',
        width: '90%',
      })}
      disableEscapeKeyDown={false}
      onClose={() => setOpen(false)}
      open={open}
    >
      <DialogHeader>
        <DialogTitle typography="body1" bold>
          Update SAML application
        </DialogTitle>
      </DialogHeader>

      <DialogContent
        maxHeight={'90%'}
        overflow={'auto'}
        bg={theme.colors.levels.sunken}
      >
        {content()}
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary disabled={false} onClick={() => setOpen(false)}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

function samlResponseToAgentMeta(resp: SamlIdpServiceProvider) {
  let samlMeta: SamlMeta = {
    samlGeneric: resp,
  };
  if (resp.spec.preset === SamlServiceProviderPreset.GcpWorkforce) {
    samlMeta.samlGcpWorkforce = {
      isAutoConfig: true,
      orgId: '' /* we do not store the organization Id */,
      poolName: poolNameFromEntityId(resp.spec.entity_id),
      poolProviderName: resp.metadata.name,
    };
  }
  return samlMeta;
}

function poolNameFromEntityId(entityId: string): string {
  try {
    // Expected format of the entityId:
    // https://iam.googleapis.com/locations/global/workforcePools/pool_name/providers/pool_provider_name
    return new URL(entityId).pathname.split('/')[4];
  } catch {
    // While a URL is expected, user may have misconfigured
    // the field so we'll just swallow an error here and return
    // an empty poolName string.
    return '';
  }
}
