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

import { consoleRoutePatterns } from 'teleport/config';
import { ParticipantMode } from 'teleport/services/session';

import ConsoleContext from './consoleContext';

export default function useRouting(ctx: ConsoleContext) {
  const { pathname, search } = useLocation();
  const { clusterId } = useParams<{ clusterId: string }>();
  const sshRouteMatch = useMatch(consoleRoutePatterns.consoleConnect);
  const kubeExecRouteMatch = useMatch(consoleRoutePatterns.kubeExec);
  const nodesRouteMatch = useMatch(consoleRoutePatterns.consoleNodes);
  const joinSshRouteMatch = useMatch(consoleRoutePatterns.consoleSession);
  const joinKubeExecRouteMatch = useMatch(consoleRoutePatterns.kubeExecSession);
  const dbConnectMatch = useMatch(consoleRoutePatterns.dbConnect);

  // Ensure that each URL has corresponding document
  useMemo(() => {
    if (ctx.getActiveDocId(pathname) !== -1) {
      return;
    }

    const participantMode = getParticipantMode(search);

    // When no document matches current URL that means we need to
    // create one base on URL parameters.
    if (sshRouteMatch) {
      ctx.addSshDocument(sshRouteMatch.params);
    } else if (joinSshRouteMatch) {
      ctx.addSshDocument({
        ...joinSshRouteMatch.params,
        mode: participantMode,
      });
    } else if (nodesRouteMatch) {
      ctx.addNodeDocument(nodesRouteMatch.params.clusterId || clusterId);
    } else if (kubeExecRouteMatch) {
      ctx.addKubeExecDocument(kubeExecRouteMatch.params);
    } else if (joinKubeExecRouteMatch) {
      ctx.addKubeExecDocument({
        ...joinKubeExecRouteMatch.params,
        kubeId: '',
        mode: participantMode,
      });
    } else if (dbConnectMatch) {
      ctx.addDbDocument(dbConnectMatch.params);
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
