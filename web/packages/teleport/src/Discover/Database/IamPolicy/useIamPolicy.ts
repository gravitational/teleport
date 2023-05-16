/**
 * Copyright 2022 Gravitational, Inc.
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
import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContext from 'teleport/teleportContext';
import { useDiscover } from 'teleport/Discover/useDiscover';

import type { AgentStepProps } from '../../types';
import type { DatabaseIamPolicyResponse } from 'teleport/services/databases';

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
