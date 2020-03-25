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

import { useParams } from 'react-router';
import React from 'react';
import styled from 'styled-components';
import { withState } from 'shared/hooks';
import SplitPane from 'shared/components/SplitPane';
import { Danger } from 'design/Alert';
import { Indicator, Flex, Text, Box } from 'design';
import cfg from 'teleport/config';
import TtyPlayer, {
  StatusEnum as TtyStatusEnum,
} from 'teleport/lib/term/ttyPlayer';
import EventProvider from 'teleport/lib/term/ttyPlayerEventProvider';
import { ProgressBarTty } from './../player/ProgressBar';
import Xterm from './../player/Xterm';
import BpfPlayer from './BpfPlayer';
import SwitchMode, { ModeEnum } from './SwitchMode';

export function Player(props) {
  const { url, bpfEvents = [] } = props;
  const tty = React.useMemo(() => {
    return props.tty || new TtyPlayer(new EventProvider({ url }));
  }, [url]);

  const [, setStatus] = React.useState(tty.status);
  const [viewMode, setViewMode] = React.useState(ModeEnum.FULLSCREEN);

  function onChangeMode(value) {
    setViewMode(value);
  }

  React.useEffect(() => {
    function onChange() {
      setStatus(tty.status);
    }

    function cleanup() {
      tty.stop();
      tty.removeAllListeners();
    }

    tty.on('change', onChange);
    tty.connect();

    return cleanup;
  }, [url]);

  const { statusText, status } = tty;
  const eventCount = tty.getEventCount();
  const isError = status === TtyStatusEnum.ERROR;
  const isLoading = status === TtyStatusEnum.LOADING;

  if (isError) {
    return (
      <StatusBox>
        <Danger m={10}>{statusText || 'Error'}</Danger>
      </StatusBox>
    );
  }

  if (isLoading) {
    return (
      <StatusBox>
        <Indicator />
      </StatusBox>
    );
  }

  if (!isLoading && eventCount === 0) {
    return (
      <StatusBox>
        <Text typography="h4">
          Recording for this session is not available.
        </Text>
      </StatusBox>
    );
  }

  return (
    <StyledPlayer>
      <Flex flex="1" flexDirection="column" overflow="auto">
        <ToolBar px="3" py="2">
          <SwitchMode mx="auto" mode={viewMode} onChange={onChangeMode} />
        </ToolBar>
        <SplitPane flex="1" defaultSize="60%" overflow="auto" split={viewMode}>
          <Xterm p="2" tty={tty} />
          {viewMode !== ModeEnum.FULLSCREEN && (
            <BpfPlayer events={bpfEvents} tty={tty} split={viewMode} />
          )}
        </SplitPane>
      </Flex>
      {eventCount > 0 && <ProgressBarTty tty={tty} />}
    </StyledPlayer>
  );
}

export default withState(() => {
  const { sid, clusterId } = useParams();
  const url = cfg.getTerminalSessionUrl({ clusterId, sid });
  return {
    url,
    sid,
  };
})(Player);

const ToolBar = styled(Flex)`
  border-bottom: 1px solid ${({ theme }) => theme.colors.primary.main};
`;

const StatusBox = props => (
  <Box width="100%" textAlign="center" p={3} {...props} />
);

const StyledPlayer = styled.div`
  display: flex;
  height: 100%;
  width: 100%;
  position: absolute;
  flex-direction: column;
  flex: 1;
  justify-content: space-between;
`;
