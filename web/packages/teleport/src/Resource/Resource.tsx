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

import { useCallback } from 'react';
import { Box } from 'design';
import { useParams } from 'react-router';
import { useAsync } from 'shared/hooks/useAsync';

import { lockService } from 'teleport/services/locks';

import { CreateLockButton } from './CreateLockButton';

export function Resource() {
  const { resourceId } = useParams<{ clusterId: string; resourceId: string }>();

  const [createLockAttempt, runCreateLockAttempt] = useAsync(
    useCallback(
      async (
        lockType: string,
        targetId: string,
        message: string,
        ttl: string
      ) => {
        lockService.createLock({
          targets: { [lockType]: targetId },
          message,
          ttl,
        });
      },
      []
    )
  );

  return (
    <Box>
      <CreateLockButton
        createLock={runCreateLockAttempt}
        createLockAttempt={createLockAttempt}
        targetId={resourceId}
        lockType={'node'}
      />
    </Box>
  );
}
