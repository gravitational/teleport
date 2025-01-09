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

import { useState } from 'react';

import { Box, ButtonPrimary, ButtonSecondary, Flex, H2 } from 'design';
import * as Alerts from 'design/Alert';
import { DialogContent, DialogHeader } from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { outermostPadding } from '../spacing';

export function ClusterAdd(props: {
  onCancel(): void;
  onSuccess(clusterUri: string): void;
  prefill: { clusterAddress: string };
}) {
  const { clustersService } = useAppContext();
  const [{ status, statusText }, addCluster] = useAsync(
    async (addr: string) => {
      const proxyAddr = parseClusterProxyWebAddr(addr);
      const cluster = await clustersService.addRootCluster(proxyAddr);
      return props.onSuccess(cluster.uri);
    }
  );
  const [addr, setAddr] = useState(props.prefill.clusterAddress || '');

  return (
    <Box px={outermostPadding}>
      <Validation>
        {({ validator }) => (
          <form
            onSubmit={e => {
              e.preventDefault();
              validator.validate() && addCluster(addr);
            }}
          >
            <DialogHeader>
              <H2>Enter cluster address</H2>
            </DialogHeader>
            <DialogContent mb={0} gap={3}>
              {status === 'error' && (
                <Alerts.Danger mb={0} details={statusText}>
                  Could not add the cluster
                </Alerts.Danger>
              )}
              <FieldInput
                rule={requiredField('Cluster address is required')}
                value={addr}
                autoFocus
                mb={0}
                onChange={e => setAddr(e.target.value)}
                placeholder="teleport.example.com"
              />
              <Flex gap={3}>
                <ButtonPrimary disabled={status === 'processing'} type="submit">
                  Next
                </ButtonPrimary>
                <ButtonSecondary
                  disabled={status === 'processing'}
                  type="button"
                  onClick={e => {
                    e.preventDefault();
                    props.onCancel();
                  }}
                >
                  Cancel
                </ButtonSecondary>
              </Flex>
            </DialogContent>
          </form>
        )}
      </Validation>
    </Box>
  );
}

function parseClusterProxyWebAddr(addr: string) {
  addr = addr || '';
  if (addr.startsWith('http')) {
    const url = new URL(addr);
    return url.host;
  }

  return addr;
}
