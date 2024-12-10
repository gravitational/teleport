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

import React from 'react';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import * as Alerts from 'design/Alert';
import { ButtonIcon, ButtonPrimary, Text } from 'design';

import { Cross } from 'design/Icon';

import { RootClusterUri } from 'teleterm/ui/uri';

import { useClusterLogout } from './useClusterLogout';

export function ClusterLogout({
  clusterUri,
  onClose,
  clusterTitle,
  hidden,
}: {
  clusterTitle: string;
  clusterUri: RootClusterUri;
  hidden?: boolean;
  onClose(): void;
}) {
  const { removeCluster, status, statusText } = useClusterLogout({
    clusterUri,
  });

  async function removeClusterAndClose(): Promise<void> {
    const [, err] = await removeCluster();
    if (!err) {
      onClose();
    }
  }

  return (
    <DialogConfirmation
      open={!hidden}
      keepInDOMAfterClose
      onClose={onClose}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          removeClusterAndClose();
        }}
      >
        <DialogHeader justifyContent="space-between" mb={0}>
          <Text typography="h5" bold style={{ whiteSpace: 'nowrap' }}>
            Log out from cluster {clusterTitle}
          </Text>
          <ButtonIcon
            type="button"
            disabled={status === 'processing'}
            onClick={onClose}
            color="text.slightlyMuted"
          >
            <Cross size="medium" />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={4}>
          <Text color="text.slightlyMuted" typography="body1">
            Are you sure you want to log out?
          </Text>
          {status === 'error' && <Alerts.Danger mb={5} children={statusText} />}
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary
            kind="warning"
            disabled={status === 'processing'}
            size="large"
            block={true}
            autoFocus
            type="submit"
          >
            Log out
          </ButtonPrimary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
