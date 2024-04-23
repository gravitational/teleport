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
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog, { DialogContent } from 'design/Dialog';
import Flex from 'design/Flex';
import { useAsync } from 'shared/hooks/useAsync';

type Props = {
  onClose(): void;
  onAuthorize(): void;
};

export const AuthenticateWebDevice = ({ onAuthorize, onClose }: Props) => {
  const [attempt, run] = useAsync(async () => {
    await onAuthorize();
    onClose();
  });

  return (
    <Dialog open={true}>
      <DialogContent>
        Would you like to launch an authorized web session?
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
          Close
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
};
