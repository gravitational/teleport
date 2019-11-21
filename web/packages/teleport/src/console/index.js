/*
Copyright 2019 Gravitational, Inc.

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
import { useRouteMatch, useParams, useLocation } from 'react-router';
import cfg from 'teleport/config';
import session from 'teleport/services/session';
import history from 'teleport/services/history';
import useConsoleContext from './useConsoleContext';
import Console from './components/Console';

export default function Container() {
  const createSessionRequest = useRouteMatch(cfg.routes.consoleConnect);
  const joinSessionRequest = useRouteMatch(cfg.routes.consoleSession);
  const homeRequest = useLocation();
  const { clusterId } = useParams();
  const consoleContext = useConsoleContext();

  React.useState(() => {
    consoleContext.init({ clusterId });
  });

  // find the document which matches current URL
  const doc = consoleContext.makeActiveByUrl(homeRequest.pathname);

  React.useEffect(() => {
    if (doc) {
      return;
    }

    function newTab({ serverId, login, sid }) {
      const { url } = consoleContext.addTerminalTab({ login, serverId, sid });
      history.push(url);
    }

    if (createSessionRequest) {
      newTab(createSessionRequest.params);
    }

    if (joinSessionRequest) {
      newTab(joinSessionRequest.params);
    }
  }, [homeRequest.pathname]);

  function onSelectTab({ url }) {
    history.push(url);
  }

  function onCloseTab({ id }) {
    const nextDoc = consoleContext.closeTab(id);
    nextDoc && history.push(nextDoc.url);
  }

  function onLogout() {
    session.logout();
  }

  return (
    <Console
      onLogout={onLogout}
      onCloseTab={onCloseTab}
      onSelect={onSelectTab}
    />
  );
}
