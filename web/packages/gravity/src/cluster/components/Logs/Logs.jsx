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
import cfg from 'gravity/config';
import LogViewer from 'gravity/components/LogViewer';
import LogProvider from './LogProvider';
import QueryEditorBasic from './QueryEditorBasic';
import { getUrlParameter } from 'gravity/services/history';
import * as Alerts from 'design/Alert';
import { Box, Flex, Input, ButtonSecondary } from 'design';
import { Cog } from 'design/Icon';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import LogForwarderDialog, { LogforwarderStore } from './LogForwarderDialog';
import { useAttempt, withState } from 'shared/hooks';

export function ClusterLogs(props) {
  const {
    query,
    queryUrl,
    isSettingsOpen,
    refreshCount,
    logForwarderStore,
    attempt,
    onSearch,
    onRefresh,
    onOpenSettings,
    onCloseSettings,
  } = props;

  const { isProcessing } = attempt;

  return (
    <FeatureBox pb="0">
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr={3}>
          Logs
        </FeatureHeaderTitle>
        <StyledInputBar>
          <QueryEditorBasic key={query} query={query} onChange={onSearch}/>
          <StyledButton mr="3" secondary onClick={onRefresh}>
            Refresh
          </StyledButton>
        </StyledInputBar>
        <Box ml="auto" flex="0 0 auto">
          <ButtonSecondary onClick={onOpenSettings} disabled={isProcessing}>
            <Cog mr="3" ml={-2} fontSize="14px"/>
            LOG FORWARDER SETTINGS
          </ButtonSecondary>
        </Box>
      </FeatureHeader>
      {attempt.isFailed &&  (
        <Alerts.Danger>
          {attempt.message}
        </Alerts.Danger>
      )}
      <Flex mx={-6} mb={-4} px="2" height="100%" bg="bgTerminal">
        <LogViewer autoScroll={true} provider={
            <LogProvider key={refreshCount} queryUrl={queryUrl} />
        }/>
      </Flex>
        { isSettingsOpen && <LogForwarderDialog store={logForwarderStore} onClose={onCloseSettings}/> }
    </FeatureBox>
  )
}

const StyledInputBar = styled.div`
  display: flex;
  align-items: center;
  width: 60%;
  ${Input}{
    border-bottom-right-radius: 0;
    border-top-right-radius: 0;
  }
`

const StyledButton = styled(ButtonSecondary)`
  border-bottom-right-radius: 0;
  border-top-right-radius: 0;
`

export default withState(({ match, history }) => {
  const { siteId } = match.params;
  const query = getUrlParameter('query', history.location.search);
  const queryUrl = cfg.getSiteLogAggregatorUrl(siteId, query);

  // hooks
  const logForwarderStore = React.useMemo(() => new LogforwarderStore(), []);
  const [ attempt, attemptActions ] = useAttempt();
  const [ refreshCount, setRefreshCount ] = React.useState(0);
  const [ isSettingsOpen, setIsSettingOpen ] = React.useState(false);

  function onSearch(query) {
    history.location.search = `query=${query}`;
    history.push(history.location);
  }

  function onRefresh() {
    setRefreshCount(refreshCount+1);
  }

  function onOpenSettings() {
    attemptActions.do(() => {
      return logForwarderStore.fetch().then(() => setIsSettingOpen(true))
    })
  }

  function onCloseSettings() {
    setIsSettingOpen(false)
  }

  return {
    query,
    queryUrl,
    logForwarderStore,
    isSettingsOpen,
    refreshCount,
    attempt,
    onSearch,
    onRefresh,
    onOpenSettings,
    onCloseSettings,
}})(ClusterLogs);
