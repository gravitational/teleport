/*
Copyright 2015 Gravitational, Inc.

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

import reactor from 'app/reactor';
import {filter} from './getters';
import {fetchSiteEvents} from './../sessions/actions';
import { TLPT_STORED_SESSINS_FILTER_SET_RANGE } from './actionTypes';
import Logger from 'app/lib/logger';

const logger = Logger.create('Modules/Sessions');

const actions = {

  fetchSiteEventsWithinTimeRange(){
    let { start, end } = reactor.evaluate(filter);
    return _fetch(start, end);
  },

  setTimeRange(start, end){
    reactor.batch(()=>{
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, {start, end});
      _fetch(start, end);
    });
  }
}

function _fetch(start, end){
  return fetchSiteEvents(start, end)
    .fail(err => {      
      logger.error('fetching filtered set of sessions', err);
    });
}

export default actions;
