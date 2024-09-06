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

import React from 'react';
import { useRouteMatch, useParams, useLocation } from 'react-router';

import cfg, { UrlKubeExecParams, UrlSshParams } from 'teleport/config';
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

  // Ensure that each URL has corresponding document
  React.useMemo(() => {
    if (ctx.getActiveDocId(pathname) !== -1) {
      return;
    }

    // When no document matches current URL that means we need to
    // create one base on URL parameters.
    if (sshRouteMatch) {
      ctx.addSshDocument(sshRouteMatch.params);
    } else if (joinSshRouteMatch) {
      // Extract the mode param from the URL if it is present.
      const searchParams = new URLSearchParams(search);
      const mode = searchParams.get('mode');
      if (mode) {
        joinSshRouteMatch.params.mode = mode as ParticipantMode;
      }
      ctx.addSshDocument(joinSshRouteMatch.params);
    } else if (nodesRouteMatch) {
      ctx.addNodeDocument(clusterId);
    } else if (kubeExecRouteMatch) {
      ctx.addKubeExecDocument(kubeExecRouteMatch.params);
    }
  }, [ctx, pathname]);

  return {
    clusterId,
    activeDocId: ctx.getActiveDocId(pathname),
  };
}
