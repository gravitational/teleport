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
import { Flex, Text } from 'design';
import AjaxPoller from 'gravity/components/AjaxPoller';

const POLL_INTERVAL = 5000; // every 5 sec

export default function Progress(props){
  const { progress, opId, onFetch } = props;

  function onRefresh(){
    return onFetch(opId)
  }

  const $poller = <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
  // if progress is not available yet, just keep polling
  if(!progress){
    return $poller;
  }

  const { step, message } = progress;
  const value = (step + 1) * 10;
  return (
    <>
      <Flex flexDirection="column">
        <StyledProgressBar height="10px"
          mb="1"
          value={value}
          isCompleted={false}
        >
          <span/>
        </StyledProgressBar>
        <Text typography="body2">
          {`step ${step+1} out of 10 - ${message}`}
        </Text>
      </Flex>
      {$poller}
    </>
  )
}

const StyledProgressBar = styled(Flex)`
  align-items: center;
  flex-shrink: 0;
  background-color: ${ ({theme}) => theme.colors.primary.lighter};
  border-radius: 12px;
  > span {
    border-radius: 12px;
    ${({theme, value}) => `
      height: 100%;
      width: ${value}%;
      background-color: ${theme.colors.success};
    `}
  }
`;