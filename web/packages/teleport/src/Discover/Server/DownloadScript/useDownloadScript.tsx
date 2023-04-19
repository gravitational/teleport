/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState, useEffect } from 'react';
import {
  addMinutes,
  intervalToDuration,
  differenceInMilliseconds,
} from 'date-fns';
import useAttempt from 'shared/hooks/useAttemptNext';

import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';
import TeleportContext from 'teleport/teleportContext';

import { AgentStepProps } from '../../types';

import type { JoinToken } from 'teleport/services/joinToken';

const FIVE_MINUTES = 5;
const THREE_SECONDS_IN_MS = 3000;
const ONE_SECOND_IN_MS = 1000;

export function useDownloadScript({ ctx, props }: Props) {
  const { attempt, run, setAttempt } = useAttempt('processing');
  const [joinToken, setJoinToken] = useState<JoinToken>();
  const [pollState, setPollState] = useState<PollState>('polling');

  // TODO (lisa) extract count down logic into it's own component.
  const [countdownTime, setCountdownTime] = useState<CountdownTime>({
    minutes: 5,
    seconds: 0,
  });

  // Responsible for initial join token fetch.
  useEffect(() => {
    fetchJoinToken();
  }, []);

  // Responsible for initiating polling and
  // timeout on join token change.
  useEffect(() => {
    if (!joinToken) return;

    setPollState('polling');

    // abortController is required to cancel in flight request.
    const abortController = new AbortController();
    const abortSignal = abortController.signal;
    let timeoutId;
    let pollingIntervalId;
    let countdownIntervalId;

    // countdownEndDate takes current date and adds 5 minutes to it.
    let countdownEndDate = addMinutes(new Date(), FIVE_MINUTES);

    // inFlightReq is a flag to prevent another fetch request when a
    // previous fetch request is still in progress. May happen when a
    // previous fetch request is taking longer than the polling
    // interval time.
    let inFlightReq;

    function cleanUp() {
      clearInterval(pollingIntervalId);
      clearInterval(countdownIntervalId);
      clearTimeout(timeoutId);
      setCountdownTime({ minutes: 5, seconds: 0 });
      // Cancel any in flight request.
      abortController.abort();
    }

    function fetchNodeMatchingRefResourceId() {
      if (inFlightReq) return;

      inFlightReq = ctx.nodeService
        .fetchNodes(
          ctx.storeUser.getClusterId(),
          {
            search: `${INTERNAL_RESOURCE_ID_LABEL_KEY} ${joinToken.internalResourceId}`,
            limit: 1,
          },
          abortSignal
        )
        .then(res => {
          if (res.agents.length > 0) {
            setPollState('success');
            props.updateAgentMeta({
              ...props.agentMeta,
              // Node is an oddity in that the hostname is the more
              // user friendly text.
              resourceName: res.agents[0].hostname,
              node: res.agents[0],
            });
            cleanUp();
          }
        })
        // Polling related errors are ignored.
        // The most likely cause of error would be network issues
        // and aborting in flight request.
        .catch(() => {})
        .finally(() => {
          inFlightReq = null; // reset flag
        });
    }

    function updateCountdown() {
      const start = new Date();
      const end = countdownEndDate;
      const duration = intervalToDuration({ start, end });

      if (differenceInMilliseconds(end, start) <= 0) {
        setPollState('error');
        cleanUp();
        return;
      }

      setCountdownTime({
        minutes: duration.minutes,
        seconds: duration.seconds,
      });
    }

    // Set a countdown in case polling continuosly produces
    // no results. Which means there is either a network error,
    // script is ran unsuccessfully, script has not been ran,
    // or resource cannot connect to cluster.
    countdownIntervalId = setInterval(
      () => updateCountdown(),
      ONE_SECOND_IN_MS
    );

    // Start the poller to discover the resource just added.
    pollingIntervalId = setInterval(
      () => fetchNodeMatchingRefResourceId(),
      THREE_SECONDS_IN_MS
    );

    return () => {
      cleanUp();
    };
  }, [joinToken]);

  function fetchJoinToken() {
    run(() =>
      ctx.joinTokenService.fetchJoinToken(['Node'], 'token').then(token => {
        // Probably will never happen, but just in case, otherwise
        // querying for the resource can return a false positive.
        if (!token.internalResourceId) {
          setAttempt({
            status: 'failed',
            statusText:
              'internal resource ID is required to discover the newly added resource, but none was provided',
          });
          return;
        }
        setJoinToken(token);
      })
    );
  }

  function regenerateScriptAndRepoll() {
    fetchJoinToken();
  }

  return {
    attempt,
    joinToken,
    nextStep: props.nextStep,
    pollState,
    regenerateScriptAndRepoll,
    countdownTime,
  };
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

type PollState = 'polling' | 'success' | 'error';

export type CountdownTime = {
  minutes: number;
  seconds: number;
};

export type State = ReturnType<typeof useDownloadScript>;
