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
import styled from 'styled-components';
import { Flex } from 'design';
import AjaxPoller from 'teleport/components/AjaxPoller';
import { colors } from './colors';
import Tabs from './Tabs';
import useConsoleContext, { useStoreDocs } from './../useConsoleContext';
import ActionBar from './ActionBar';
import Document from './Document';

const POLL_INTERVAL = 3000; // every 3 sec

export default function Console(props) {
  const { onLogout, onCloseTab, onSelect } = props;
  const storeDocs = useStoreDocs();
  const consoleContext = useConsoleContext();
  const { active, items } = storeDocs.state;
  const hasActiveSessions = storeDocs.hasActiveTerminalSessions();

  function onRefresh() {
    return consoleContext.refreshParties();
  }

  const docs = items.map(doc => {
    return <Document doc={doc} visible={doc.id === active} key={doc.id} />;
  });

  return (
    <StyledConsole>
      <Flex bg="primary.dark" height="38px">
        <Tabs
          flex="1"
          items={items}
          onClose={onCloseTab}
          onSelect={onSelect}
          activeTab={active}
        />
        <ActionBar tabId={active} onLogout={onLogout} />
      </Flex>
      {docs}
      {hasActiveSessions && (
        <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
      )}
    </StyledConsole>
  );
}

const StyledConsole = styled.div`
  background-color: ${colors.bgTerminal};
  bottom: 0;
  left: 0;
  position: absolute;
  right: 0;
  top: 0;
  display: flex;
  flex-direction: column;
`;
