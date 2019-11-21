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
import { withState } from 'shared/hooks';
import { Indicator, Flex, Text, Box, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import * as Alerts from 'design/Alert';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import Xterm from './Xterm/Xterm';
import { useStore } from 'shared/libs/stores';
import TerminalStore from './storeTerminal';

export function Terminal(props) {
  const {
    terminal,
    onReplay,
    onSessionStart,
    onSessionEnd,
    onDisconnect,
  } = props;
  const termRef = React.useRef();

  if (terminal.isLoading()) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  if (terminal.isError()) {
    return (
      <Alerts.Danger m={10} as={Flex} alignSelf="baseline" flex="1">
        Connection error: {terminal.state.statusText}
      </Alerts.Danger>
    );
  }

  if (terminal.isNotFound()) {
    return <SidNotFoundError onReplay={onReplay} />;
  }

  const termConfig = terminal.getTtyConfig();

  return (
    <Flex
      flexDirection="column"
      height="100%"
      width="100%"
      px="2"
      style={{ overflow: 'auto' }}
    >
      <Xterm
        ref={termRef}
        onDisconnect={onDisconnect}
        onSessionEnd={onSessionEnd}
        onSessionStart={onSessionStart}
        termConfig={termConfig}
      />
    </Flex>
  );
}

const SidNotFoundError = ({ onReplay }) => (
  <Box my={10} mx="auto" width="300px">
    <Text typography="h4" mb="3" textAlign="center">
      The session is no longer active
    </Text>
    <ButtonSecondary block secondary onClick={onReplay}>
      <Icons.CirclePlay fontSize="5" mr="2" /> Replay Session
    </ButtonSecondary>
  </Box>
);

export default withState(props => {
  const { sid, clusterId, serverId, login } = props;
  const terminal = React.useMemo(() => {
    const store = new TerminalStore();
    store.init({
      sid,
      clusterId,
      serverId,
      login,
    });

    return store;
  }, []);

  useStore(terminal);

  function onDisconnect() {
    props.onDisconnect(false);
  }

  function onSessionStart() {
    props.onConnect(terminal.state.session);
  }

  function onSessionEnd() {
    onDisconnect(true);
  }

  function onOpenPlayer() {
    const routeUrl = cfg.getSessionAuditRoute({ clusterId, sid });
    history.push(routeUrl);
  }

  return {
    terminal,
    onReplay: onOpenPlayer,
    onDisconnect,
    onSessionStart,
    onSessionEnd,
  };
})(Terminal);
