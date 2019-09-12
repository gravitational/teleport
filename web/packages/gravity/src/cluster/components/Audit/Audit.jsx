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
import { EVENT_MAX_LIMIT, fetchEvents, fetchLatest } from 'gravity/cluster/flux/events/actions';
import { getters } from 'gravity/cluster/flux/events';
import AjaxPoller from 'gravity/components/AjaxPoller';
import { useFluxStore } from 'gravity/components/nuclear';
import { useAttempt, withState } from 'shared/hooks';
import { Danger } from 'design/Alert';
import RangePicker, { getRangeOptions } from './RangePicker';
import EventList from './EventList';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import InputSearch from './../components/InputSearch';

const POLL_INTERVAL = 5000; // every 5 sec

export class Audit extends React.Component {

  onRangeChange = range => {
    this.props.onRangeChange(range)
    this.props.onFetch(range);
  }

  componentDidMount(){
    this.props.onFetch(this.props.range);
  }

  render() {
    const { store, attempt, searchValue, range, rangeOptions, onFetchLatest, onSearchChange } = this.props;
    const { from, to } = range;
    const events = store.getEvents();
    const { overflow } = store;
    const { isFailed, message } = attempt;

    const filtered = events.filter(
      item => moment(item.time).isBetween(from, to));

    return (
      <FeatureBox>
        <FeatureHeader alignItems="center">
          <FeatureHeaderTitle mr="5">
            Audit Log
          </FeatureHeaderTitle>
          <InputSearch bg="primary.light" mr="3" autoFocus onChange={onSearchChange}/>
          <RangePicker  ml="auto"
            value={range}
            options={rangeOptions}
            onChange={this.onRangeChange}
          />
        </FeatureHeader>
        { overflow && <Danger> Number of events retrieved for specified date range was exceeded the maximum limit of {EVENT_MAX_LIMIT} </Danger> }
        { isFailed && <Danger> {message} </Danger> }
        <EventList
          search={searchValue}
          events={filtered}
        />
        <AjaxPoller time={POLL_INTERVAL} onFetch={onFetchLatest} />
      </FeatureBox>
    )
  }
}

function mapState(){
  const store = useFluxStore(getters.store);
  const rangeOptions = React.useMemo(() => getRangeOptions(), []);
  const [ attempt, attemptActions ] = useAttempt();
  const [ searchValue, setSearchValue ] = React.useState('');
  const [ range , setRange ] = React.useState(rangeOptions[0]);

  function onFetch({from, to}){
    attemptActions.do(() => {
      return fetchEvents({start: from, end: to});
    })
  }

  function onRangeChange(range){
    setRange(range);
  }

  return {
    store,
    searchValue,
    range,
    rangeOptions,
    attempt,
    attemptActions,
    onFetch,
    onRangeChange,
    onSearchChange: setSearchValue,
    onFetchLatest: fetchLatest,
  }
}

export default withState(mapState)(Audit);