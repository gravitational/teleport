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

import cfg from 'teleport/config';
import TeleportContext from 'teleport/teleportContext';
import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import {
  resourceKindToJoinRole,
  ResourceKind,
} from 'teleport/Discover/Shared/ResourceKind';

import { DbMeta, useDiscover } from '../../useDiscover';

import type { AgentStepProps } from '../../types';

export function useMutualTls({ ctx, props }: Props) {
  const { attempt, run } = useAttempt('');

  const { emitErrorEvent } = useDiscover();
  const { joinToken: prevFetchedJoinToken } = useJoinTokenSuspender(
    ResourceKind.Database
  );
  const [joinToken, setJoinToken] = useState(prevFetchedJoinToken);
  const meta = props.agentMeta as DbMeta;
  const clusterId = ctx.storeUser.getClusterId();

  useEffect(() => {
    // A joinToken may not exist if the previous step (install db service)
    // was skipped due to an already existing db service that was able
    // to pick up the newly created database resource (from matching labels).
    if (!joinToken) {
      // We don't need to preserve this token, it's used
      // as a auth bearer token and gets deleted
      // right after it gets checked for validity.
      // This also means it invalidates the existing joinToken.
      run(() =>
        ctx.joinTokenService
          .fetchJoinToken({
            roles: [resourceKindToJoinRole(ResourceKind.Database)],
            method: 'token',
          })
          .then(setJoinToken)
          .catch((error: Error) => {
            emitErrorEvent(`error with fetching join token: ${error.message}`);
            throw error;
          })
      );
    }
    // Ensure runs only once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function onNextStep(caCert: string) {
    if (!caCert) {
      props.nextStep();
      return;
    }

    run(() =>
      ctx.databaseService
        .updateDatabase(clusterId, {
          name: meta.db.name,
          caCert,
        })
        .then(() => props.nextStep())
        .catch((error: Error) => {
          emitErrorEvent(
            `error with update database with caCert: ${error.message}`
          );
          throw error;
        })
    );
  }

  const access = ctx.storeUser.getDatabaseAccess();
  return {
    attempt,
    onNextStep,
    canUpdateDatabase: access.edit,
    curlCmd: generateSignCertificateCurlCommand(
      clusterId,
      meta.db.hostname,
      joinToken?.id
    ),
    dbEngine: props.resourceSpec.dbMeta.engine,
  };
}

function generateSignCertificateCurlCommand(
  clusterId: string,
  hostname: string,
  token: string
) {
  if (!token) return '';

  const requestUrl = cfg.getDatabaseSignUrl(clusterId);
  const requestData = JSON.stringify({ hostname });

  // curl flag -OJ  makes curl use the file name
  // defined from the response header.
  return `curl ${cfg.baseUrl}/${requestUrl}\
 -d '${requestData}'\
 -H 'Authorization: Bearer ${token}'\
 -H 'Content-Type: application/json' -OJ;\
 tar -xvf teleport_mTLS_${hostname}.tar.gz
  `;
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useMutualTls>;
