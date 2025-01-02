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

import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import { useDiscover } from 'teleport/Discover/useDiscover';
import type { DatabaseIamPolicyResponse } from 'teleport/services/databases';
import TeleportContext from 'teleport/teleportContext';

import type { AgentStepProps } from '../../types';

export function useIamPolicy({ ctx, props }: Props) {
  const { attempt, run } = useAttempt('');
  const { emitErrorEvent } = useDiscover();

  const [iamPolicy, setIamPolicy] = useState<DatabaseIamPolicyResponse>();

  useEffect(() => {
    fetchIamPolicy();

    // Ensure runs only once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function fetchIamPolicy() {
    const clusterId = ctx.storeUser.getClusterId();

    run(() =>
      ctx.databaseService
        .fetchDatabaseIamPolicy(clusterId, props.agentMeta.resourceName)
        .then(setIamPolicy)
        .catch((error: Error) => {
          emitErrorEvent(error.message);
          throw error;
        })
    );
  }

  return {
    attempt,
    nextStep: props.nextStep,
    iamPolicy,
    fetchIamPolicy,
    // Creates a unique policy name since db resource name's are unique.
    iamPolicyName: `TeleportDatabaseAccess_${props.agentMeta.resourceName}`,
  };
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useIamPolicy>;
