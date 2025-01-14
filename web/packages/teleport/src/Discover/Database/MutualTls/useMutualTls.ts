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

import cfg from 'teleport/config';
import {
  ResourceKind,
  resourceKindToJoinRole,
} from 'teleport/Discover/Shared/ResourceKind';
import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import TeleportContext from 'teleport/teleportContext';

import type { AgentStepProps } from '../../types';
import { DbMeta, useDiscover } from '../../useDiscover';

export function useMutualTls({ ctx, props }: Props) {
  const { attempt, run } = useAttempt('');

  const { emitErrorEvent } = useDiscover();
  const { joinToken: prevFetchedJoinToken } = useJoinTokenSuspender({
    resourceKinds: [ResourceKind.Database],
  });
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
  const ttl = cfg.getDatabaseCertificateTTL();
  const requestData = JSON.stringify({ hostname, ttl });

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
