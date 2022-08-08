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
import useAttempt from 'shared/hooks/useAttemptNext';

import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';

import { DiscoverContext } from '../discoverContext';
import { AgentStepProps } from '../types';

import type { JoinToken } from 'teleport/services/joinToken';

const FIVE_MINUTES_IN_MS = 300000;
const THREE_SECONDS_IN_MS = 3000;

export function useDownloadScript({ ctx, props }: Props) {
  const { attempt, run, setAttempt } = useAttempt('processing');
  const [joinToken, setJoinToken] = useState<JoinToken>();
  const [pollState, setPollState] = useState<PollState>('polling');

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
    let intervalId;

    // inFlightReq is a flag to prevent another fetch request when a
    // previous fetch request is still in progress. May happen when a
    // previous fetch request is taking longer than the polling
    // interval time.
    let inFlightReq;

    function cleanUp() {
      clearInterval(intervalId);
      clearTimeout(timeoutId);
      // Cancel any in flight request.
      abortController.abort();
    }

    function fetchNodeMatchingRefResourceId() {
      if (inFlightReq) return;

      inFlightReq = ctx.nodesService
        .fetchNodes(
          ctx.clusterId,
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

    // Start the poller to discover the resource just added.
    intervalId = setInterval(
      () => fetchNodeMatchingRefResourceId(),
      THREE_SECONDS_IN_MS
    );

    // Set a timeout in case polling continuosly produces
    // no results. Which means there is either a network error,
    // script is ran unsuccessfully, script has not been ran,
    // or resource cannot connect to cluster.
    timeoutId = setTimeout(() => {
      setPollState('error');
      cleanUp();
    }, FIVE_MINUTES_IN_MS);

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
  };
}

type Props = {
  ctx: DiscoverContext;
  props: AgentStepProps;
};

type PollState = 'polling' | 'success' | 'error';

export type State = ReturnType<typeof useDownloadScript>;
