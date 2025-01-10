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

import { useState } from 'react';

import { Box, Indicator } from 'design';
import { Danger } from 'design/Alert';
import { ClusterDropdown } from 'shared/components/ClusterDropdown/ClusterDropdown';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { CtaEvent } from 'teleport/services/userEvent';
import useStickerClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import SessionList from './SessionList';
import useSessions from './useSessions';

export function SessionsContainer() {
  const ctx = useTeleport();
  const { clusterId } = useStickerClusterId();
  const state = useSessions(ctx, clusterId);
  return <Sessions {...state} />;
}

export function Sessions(props: ReturnType<typeof useSessions>) {
  const {
    ctx,
    attempt,
    sessions,
    showActiveSessionsCTA,
    showModeratedSessionsCTA,
    clusterId,
  } = props;
  const [errorMessage, setErrorMessage] = useState('');

  return (
    <FeatureBox>
      <FeatureHeader
        alignItems="center"
        justifyContent="space-between"
        css={`
          @media screen and (max-width: 800px) {
            flex-direction: column;
            height: auto;
            gap: 10px;
            margin: 0 0 10px 0;
            padding-bottom: 10px;
            justify-content: center;
          }
        `}
      >
        <FeatureHeaderTitle>Active Sessions</FeatureHeaderTitle>
        {showActiveSessionsCTA && (
          <Box>
            <ButtonLockedFeature
              height="36px"
              event={CtaEvent.CTA_ACTIVE_SESSIONS}
            >
              Join Active Sessions With Teleport Enterprise
            </ButtonLockedFeature>
          </Box>
        )}
      </FeatureHeader>
      {!errorMessage && (
        <ClusterDropdown
          clusterLoader={ctx.clusterService}
          clusterId={clusterId}
          onError={setErrorMessage}
          mb={2}
        />
      )}
      {errorMessage && <Danger>{errorMessage}</Danger>}
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
          showModeratedSessionsCTA={showModeratedSessionsCTA}
        />
      )}
    </FeatureBox>
  );
}
