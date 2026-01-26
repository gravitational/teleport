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

import { useMemo } from 'react';
import { useLocation, useMatch, useParams } from 'react-router';

import cfg, { consoleRoutePatterns } from 'teleport/config';
import { ParticipantMode } from 'teleport/services/session';

import ConsoleContext from './consoleContext';

export default function useRouting(ctx: ConsoleContext) {
  const { pathname, search } = useLocation();
  const { clusterId: currentClusterId } = useParams<'clusterId'>();
  const sshRouteMatch = useMatch(consoleRoutePatterns.consoleConnect);
  const kubeExecRouteMatch = useMatch(consoleRoutePatterns.kubeExec);
  const nodesRouteMatch = useMatch(consoleRoutePatterns.consoleNodes);
  const joinSshRouteMatch = useMatch(consoleRoutePatterns.consoleSession);
  const joinKubeExecRouteMatch = useMatch(consoleRoutePatterns.kubeExecSession);
  const dbConnectMatch = useMatch(consoleRoutePatterns.dbConnect);

  const clusterId =
    currentClusterId ||
    sshRouteMatch?.params.clusterId ||
    kubeExecRouteMatch?.params.clusterId ||
    nodesRouteMatch?.params.clusterId ||
    joinSshRouteMatch?.params.clusterId ||
    joinKubeExecRouteMatch?.params.clusterId ||
    dbConnectMatch?.params.clusterId ||
    cfg.proxyCluster;

  // Ensure that each URL has corresponding document
  useMemo(() => {
    if (ctx.getActiveDocId(pathname) !== -1) {
      return;
    }

    const participantMode = getParticipantMode(search);

    // When no document matches current URL that means we need to
    // create one base on URL parameters.
    if (sshRouteMatch) {
      const { clusterId, login, serverId } = sshRouteMatch.params;
      if (clusterId && login && serverId) {
        ctx.addSshDocument({ clusterId, login, serverId });
      }
    } else if (joinSshRouteMatch) {
      const { clusterId, sid } = joinSshRouteMatch.params;
      if (clusterId && sid) {
        ctx.addSshDocument({ clusterId, sid, mode: participantMode });
      }
    } else if (nodesRouteMatch) {
      ctx.addNodeDocument(clusterId);
    } else if (kubeExecRouteMatch) {
      const { clusterId, kubeId } = kubeExecRouteMatch.params;
      if (clusterId && kubeId) {
        ctx.addKubeExecDocument({ clusterId, kubeId });
      }
    } else if (joinKubeExecRouteMatch) {
      const { clusterId, sid } = joinKubeExecRouteMatch.params;
      if (clusterId && sid) {
        ctx.addKubeExecDocument({
          clusterId,
          kubeId: '',
          sid,
          mode: participantMode,
        });
      }
    } else if (dbConnectMatch) {
      const { clusterId, serviceName } = dbConnectMatch.params;
      if (clusterId && serviceName) {
        ctx.addDbDocument({ clusterId, serviceName });
      }
    }
  }, [ctx, pathname]);

  return {
    clusterId,
    activeDocId: ctx.getActiveDocId(pathname),
  };
}

function getParticipantMode(search: string): ParticipantMode | undefined {
  const searchParams = new URLSearchParams(search);
  const mode = searchParams.get('mode');
  if (mode === 'observer' || mode === 'moderator' || mode === 'peer') {
    return mode;
  }
}
