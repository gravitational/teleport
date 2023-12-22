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
import styled from 'styled-components';
import { Danger } from 'design/Alert';
import { Indicator, Flex, Text, Box } from 'design';

import cfg from 'teleport/config';
import TtyPlayer, {
  StatusEnum as TtyStatusEnum,
} from 'teleport/lib/term/ttyPlayer';
import EventProvider from 'teleport/lib/term/ttyPlayerEventProvider';

import { ProgressBarTty } from './ProgressBar';
import Xterm from './Xterm';

export default function Player({ sid, clusterId }) {
  const { tty } = useSshPlayer(clusterId, sid);
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
        <Xterm tty={tty} />
      </Flex>
      {eventCount > 0 && <ProgressBarTty tty={tty} />}
    </StyledPlayer>
  );
}

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

function useSshPlayer(clusterId: string, sid: string) {
  const tty = React.useMemo(() => {
    const prefixUrl = cfg.getSshPlaybackPrefixUrl({ clusterId, sid });
    return new TtyPlayer(new EventProvider({ url: prefixUrl }));
  }, [sid, clusterId]);

  // to trigger re-render when tty state changes
  const [, rerender] = React.useState(tty.status);

  React.useEffect(() => {
    function onChange() {
      // trigger rerender when status changes
      rerender(tty.status);
    }

    function cleanup() {
      tty.stop();
      tty.removeAllListeners();
    }

    tty.on('change', onChange);
    tty.connect().then(() => {
      tty.play();
    });

    return cleanup;
  }, [tty]);

  return {
    tty,
  };
}
