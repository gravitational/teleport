/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Alert } from 'design/Alert';
import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import Dialog, { DialogContent } from 'design/Dialog';
import Flex from 'design/Flex';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

export const AuthenticateWebDevice = ({
  hidden,
  onAuthorize,
  onClose,
  onCancel,
  rootClusterUri,
}: {
  rootClusterUri: RootClusterUri;
  onCancel(): void;
  onClose(): void;
  onAuthorize(): Promise<void>;
  hidden?: boolean;
}) => {
  const [attempt, run] = useAsync(async () => {
    await onAuthorize();
    onClose();
  });
  const { clustersService } = useAppContext();
  const clusterName =
    clustersService.findCluster(rootClusterUri)?.name ||
    routing.parseClusterName(rootClusterUri);

  return (
    <Dialog open={!hidden} keepInDOMAfterClose>
      {/* 400px was used as a way to do our best to get clusterName as the first item on the second line */}
      <DialogContent maxWidth="400px">
        <Text>
          Would you like to authorize a device trust web session for{' '}
          <b>{clusterName}</b>?
        </Text>
      </DialogContent>
      {attempt.status === 'error' && (
        <Alert details={attempt.statusText}>
          Could not authorize the session
        </Alert>
      )}
      <Flex flexDirection="column">
        <ButtonPrimary
          disabled={attempt.status === 'processing'}
          block={true}
          onClick={run}
          mb={3}
        >
          Authorize session
        </ButtonPrimary>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onCancel}
        >
          Continue without device trust
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
};
