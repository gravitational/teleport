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

import { ExternalAuditStorageCta } from '@gravitational/teleport/src/components/ExternalAuditStorageCta';
import { ClusterDropdown } from 'teleport/components/ClusterDropdown/ClusterDropdown';
import RangePicker from 'teleport/components/EventRangePicker';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import EventList from './EventList';
import useAuditEvents, { State } from './useAuditEvents';

export function AuditContainer() {
  const teleCtx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const state = useAuditEvents(teleCtx, clusterId);
  return <Audit {...state} />;
}

export function Audit(props: State) {
  const {
    range,
    setRange,
    rangeOptions,
    events,
    clusterId,
    onFetchNext,
    error,
    onFetchPrev,
    isSuccess,
    isLoadingPage,
    search,
    setSearch,
    sort,
    setSort,
    ctx,
  } = props;

  const [errorMessage, setErrorMessage] = useState('');

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Audit Log</FeatureHeaderTitle>
        <RangePicker
          ml="auto"
          range={range}
          ranges={rangeOptions}
          onChangeRange={setRange}
        />
      </FeatureHeader>
      <ExternalAuditStorageCta />
      {error && <Danger> {error.message} </Danger>}
      {!errorMessage && (
        <ClusterDropdown
          clusterLoader={ctx.clusterService}
          clusterId={clusterId}
          onError={setErrorMessage}
          mb={2}
        />
      )}
      {errorMessage && <Danger>{errorMessage}</Danger>}
      {isLoadingPage && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isSuccess && (
        <Box mt={2}>
          <EventList
            events={events}
            onFetchNext={onFetchNext}
            onFetchPrev={onFetchPrev}
            isLoadingPage={isLoadingPage}
            search={search}
            setSearch={setSearch}
            sort={sort}
            setSort={setSort}
          />
        </Box>
      )}
    </FeatureBox>
  );
}
