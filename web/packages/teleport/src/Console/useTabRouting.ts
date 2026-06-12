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
import { useLocation, useParams, useRouteMatch } from 'react-router';

import cfg, {
  UrlDbConnectParams,
  UrlKubeExecParams,
  UrlSshParams,
} from 'teleport/config';
import { ParticipantMode } from 'teleport/services/session';

import ConsoleContext from './consoleContext';

export default function useRouting(ctx: ConsoleContext) {
  const { pathname, search } = useLocation();
  const { clusterId } = useParams<{ clusterId: string }>();
  const sshRouteMatch = useRouteMatch<UrlSshParams>(cfg.routes.consoleConnect);
  const kubeExecRouteMatch = useRouteMatch<UrlKubeExecParams>(
    cfg.routes.kubeExec
  );
  const nodesRouteMatch = useRouteMatch(cfg.routes.consoleNodes);
  const joinSshRouteMatch = useRouteMatch<UrlSshParams>(
    cfg.routes.consoleSession
  );
  const joinKubeExecRouteMatch = useRouteMatch<UrlKubeExecParams>(
    cfg.routes.kubeExecSession
  );
  const dbConnectMatch = useRouteMatch<UrlDbConnectParams>(
    cfg.routes.dbConnect
  );

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
      joinSshRouteMatch.params.mode = participantMode;
      ctx.addSshDocument(joinSshRouteMatch.params);
    } else if (nodesRouteMatch) {
      ctx.addNodeDocument(clusterId);
    } else if (kubeExecRouteMatch) {
      ctx.addKubeExecDocument(kubeExecRouteMatch.params);
    } else if (joinKubeExecRouteMatch) {
      joinKubeExecRouteMatch.params.mode = participantMode;
      ctx.addKubeExecDocument(joinKubeExecRouteMatch.params);
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
