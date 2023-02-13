/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { useMatch, useParams, useLocation } from 'react-router';

import cfg, { UrlSshParams } from 'teleport/config';

import ConsoleContext from './consoleContext';

export default function useRouting(ctx: ConsoleContext) {
  const { pathname, search } = useLocation();
  const { clusterId } = useParams<{ clusterId: string }>();
  const sshRouteMatch = useMatch<keyof UrlSshParams, string>(
    cfg.routes.consoleConnect
  );
  const nodesRouteMatch = useMatch(cfg.routes.consoleNodes);
  const joinSshRouteMatch = useMatch<keyof UrlSshParams, string>(
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

      ctx.addSshDocument({ ...joinSshRouteMatch.params, mode });
    } else if (nodesRouteMatch) {
      ctx.addNodeDocument(clusterId);
    }
  }, [ctx, pathname]);

  return {
    clusterId,
    activeDocId: ctx.getActiveDocId(pathname),
  };
}
