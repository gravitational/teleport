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
import moment from 'moment';
import styled from 'styled-components';
import { useAttempt, withState } from 'shared/hooks';
import { Danger } from 'design/Alert';
import { Flex } from 'design';
import { borderRadius } from 'design/system';
import InputSearch from '../InputSearch';
import { useStoreEvents } from '../useAuditContext';
import EventList from './EventList';

export function AuditEvents(props) {
  const {
    events,
    maxLimit,
    overflow,
    attempt,
    searchValue,
    onSearchChange,
  } = props;

  const { isFailed, message } = attempt;

  return (
    <>
      {overflow && (
        <Danger>
          Number of events retrieved for specified date range was exceeded the
          maximum limit of {maxLimit}
        </Danger>
      )}
      {isFailed && <Danger> {message} </Danger>}
      <BorderedFlex
        bg="primary.light"
        py="3"
        px="3"
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
      >
        <InputSearch onChange={onSearchChange} />
      </BorderedFlex>
      <EventList events={events} search={searchValue} />
    </>
  );
}

function mapState(props) {
  const { range } = props;
  const store = useStoreEvents();
  const events = store.getEvents();
  const maxLimit = store.getMaxLimit();
  const { overflow } = store.state;

  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [searchValue, setSearchValue] = React.useState('');

  function onFetch({ from, to }) {
    attemptActions.do(() => {
      return store.fetchEvents({ start: from, end: to });
    });
  }

  function onFetchLatest() {
    return store.fetchLatest();
  }

  React.useEffect(() => {
    onFetch(range);
  }, [range]);

  const filtered = React.useMemo(() => {
    const { from, to } = range;
    return events.filter(item => moment(item.time).isBetween(from, to));
  }, [range, events]);

  return {
    events: filtered,
    overflow,
    searchValue,
    attempt,
    attemptActions,
    onFetch,
    onSearchChange: setSearchValue,
    onFetchLatest,
    maxLimit,
  };
}

export default withState(mapState)(AuditEvents);

const BorderedFlex = styled(Flex)`
  ${borderRadius}
`;
