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
import { Indicator, Box } from 'design';
import { Danger } from 'design/Alert';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';

import useStickerClusterId from 'teleport/useStickyClusterId';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { CtaEvent } from 'teleport/services/userEvent';

import SessionList from './SessionList';
import useSessions from './useSessions';

export default function Container() {
  const ctx = useTeleport();
  const { clusterId } = useStickerClusterId();
  const state = useSessions(ctx, clusterId);
  return <Sessions {...state} />;
}

export function Sessions(props: ReturnType<typeof useSessions>) {
  const { attempt, sessions, showActiveSessionsCTA } = props;
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Active Sessions</FeatureHeaderTitle>
        {showActiveSessionsCTA && (
          <Box width="340px">
            <ButtonLockedFeature
              height="36px"
              event={CtaEvent.CTA_ACTIVE_SESSIONS}
            >
              Join Active Sessions With Teleport Enterprise
            </ButtonLockedFeature>
          </Box>
        )}
      </FeatureHeader>
      {attempt.isFailed && <Danger>{attempt.message} </Danger>}
      {attempt.isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.isSuccess && (
        <SessionList
          sessions={sessions}
          showActiveSessionsCTA={showActiveSessionsCTA}
        />
      )}
    </FeatureBox>
  );
}
