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

import { ExternalAuditStorageCta } from '@gravitational/teleport/src/components/ExternalAuditStorageCta';
import RangePicker from 'teleport/components/EventRangePicker';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';

import RecordingsList from './RecordingsList';
import useRecordings, { State } from './useRecordings';

export function RecordingsContainer() {
  const ctx = useTeleport();
  const state = useRecordings(ctx);
  return <Recordings {...state} />;
}

export function Recordings({
  recordings,
  fetchStatus,
  fetchMore,
  range,
  setRange,
  rangeOptions,
  attempt,
  clusterId,
  ctx,
}: State) {
  const [errorMessage, setErrorMessage] = useState('');
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Session Recordings</FeatureHeaderTitle>
        <RangePicker
          ml="auto"
          range={range}
          ranges={rangeOptions}
          onChangeRange={setRange}
        />
      </FeatureHeader>
      <ExternalAuditStorageCta />
      {!errorMessage && (
        <ClusterDropdown
          clusterLoader={ctx.clusterService}
          clusterId={clusterId}
          onError={setErrorMessage}
          mb={2}
        />
      )}
      {errorMessage && <Danger>{errorMessage}</Danger>}
      {attempt.status === 'failed' && <Danger> {attempt.statusText} </Danger>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <RecordingsList
          recordings={recordings}
          clusterId={clusterId}
          fetchMore={fetchMore}
          fetchStatus={fetchStatus}
        />
      )}
    </FeatureBox>
  );
}
