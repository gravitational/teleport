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

import Alert from 'design/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, { DialogContent } from 'design/Dialog';
import Flex from 'design/Flex';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { RootClusterAppUri, routing } from 'teleterm/ui/uri';

type Props = {
  rootClusterUri: RootClusterAppUri;
  onClose(): void;
  onAuthorize(): void;
};

export const AuthenticateWebDevice = ({
  onAuthorize,
  onClose,
  rootClusterUri,
}: Props) => {
  const [attempt, run] = useAsync(async () => {
    await onAuthorize();
    onClose();
  });
  const { clustersService } = useAppContext();
  const clusterName =
    clustersService.findCluster(rootClusterUri)?.name ||
    routing.parseClusterName(rootClusterUri);

  return (
    <Dialog open={true}>
      <DialogContent maxWidth="360px">
        Would you like to launch an authorized web session for {clusterName}?
      </DialogContent>
      {attempt.status === 'error' && <Alert>{attempt.statusText}</Alert>}
      <Flex justifyContent="space-between">
        <ButtonPrimary
          disabled={attempt.status === 'processing'}
          block={true}
          textTransform="none"
          onClick={run}
          mr={4}
        >
          Launch Web Session
        </ButtonPrimary>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          textTransform="none"
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
};
