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

import {
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useState,
  type RefObject,
} from 'react';
import styled from 'styled-components';

import { Box, Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';

import cfg from 'teleport/config';
import { formatDisplayTime, StatusEnum } from 'teleport/lib/player';
import TtyPlayer from 'teleport/lib/term/ttyPlayer';
import { getAccessToken, getHostName } from 'teleport/services/api';

import ProgressBar from './ProgressBar';
import Xterm from './Xterm';

interface PlayerProps {
  sid: string;
  clusterId: string;
  durationMs: number;
  onTimeChange?: (time: number) => void;
  onToggleTimeline?: () => void;
  onToggleSidebar?: () => void;
  ref?: RefObject<PlayerHandle>;
}

export interface PlayerHandle {
  moveToTime: (time: number) => void;
}

function Player({
  sid,
  clusterId,
  durationMs,
  onTimeChange,
  onToggleSidebar,
  onToggleTimeline,
  ref,
}: PlayerProps) {
  const { tty, playerStatus, statusText, time } = useStreamingSshPlayer(
    clusterId,
    sid,
    onTimeChange
  );

  // statusText is currently only set when an error happens, so for now we can assume
  // if it is not empty, an error occured (even if the player is in COMPLETE state, which gets
  // set on close)
  const isError = playerStatus === StatusEnum.ERROR || statusText !== '';
  const isLoading = playerStatus === StatusEnum.LOADING;
  const isPlaying = playerStatus === StatusEnum.PLAYING;
  const isComplete = isError || playerStatus === StatusEnum.COMPLETE;

  const moveToTime = useCallback(
    (newTime: number) => {
      tty.move(newTime);
    },
    [tty]
  );

  // expose a moveToTime method through the ref so we can update the time from the outside
  // without having to pass it in as a prop and cause a re-render.
  useImperativeHandle(ref, () => ({
    moveToTime,
  }));

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

  return (
    <StyledPlayer>
      <Flex height="calc(100% - 56px)" flexDirection="column" px={2} py={1}>
        <Xterm tty={tty} />
      </Flex>
      <ProgressBar
        min={0}
        max={durationMs}
        current={time}
        disabled={isComplete}
        isPlaying={isPlaying}
        time={formatDisplayTime(time)}
        onRestart={() => window.location.reload()}
        onStartMove={() => tty.suspendTimeUpdates()}
        move={pos => {
          tty.resumeTimeUpdates();
          tty.move(pos);
        }}
        toggle={() => {
          isPlaying ? tty.stop() : tty.play();
        }}
        onToggleSidebar={onToggleSidebar}
        onToggleTimeline={onToggleTimeline}
      />
    </StyledPlayer>
  );
}

export { Player as default };

const StatusBox = props => (
  <Box width="100%" textAlign="center" p={3} {...props} />
);

const StyledPlayer = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  justify-content: space-between;
`;

function useStreamingSshPlayer(
  clusterId: string,
  sid: string,
  onTimeChange?: (time: number) => void
) {
  const [playerStatus, setPlayerStatus] = useState(StatusEnum.LOADING);
  const [statusText, setStatusText] = useState('');
  const [time, setTime] = useState(0);

  const tty = useMemo(() => {
    const url = cfg.api.ttyPlaybackWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':sid', sid)
      .replace(':token', getAccessToken());
    return new TtyPlayer({
      url,
      onTimeChange,
      setPlayerStatus,
      setStatusText,
      setTime,
    });
  }, [clusterId, sid, setPlayerStatus, setStatusText, onTimeChange]);

  useEffect(() => {
    tty.connect();
    tty.play();

    return () => {
      tty.stop();
      tty.removeAllListeners();
    };
  }, [tty]);

  return { tty, playerStatus, statusText, time };
}
