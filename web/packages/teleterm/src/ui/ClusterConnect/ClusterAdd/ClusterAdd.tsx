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

import React, { useState } from 'react';
import * as Alerts from 'design/Alert';
import { Box, ButtonPrimary, ButtonSecondary, H2 } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { DialogContent, DialogHeader } from 'design/Dialog';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';

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
    <Box p={4}>
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
            <DialogContent mb={2}>
              {status === 'error' && (
                <Alerts.Danger mb={5} children={statusText} />
              )}
              <FieldInput
                rule={requiredField('Cluster address is required')}
                value={addr}
                autoFocus
                onChange={e => setAddr(e.target.value)}
                placeholder="teleport.example.com"
              />
              <Box mt="5">
                <ButtonPrimary
                  disabled={status === 'processing'}
                  mr="3"
                  type="submit"
                >
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
              </Box>
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
