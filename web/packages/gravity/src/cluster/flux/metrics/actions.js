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

import $ from 'jQuery';
import service from 'gravity/cluster/services/metrics';
import reactor from 'gravity/reactor';
import Logger from 'shared/libs/logger';
import * as actionTypes from './actionTypes';

const logger = Logger.create('flux/metrics');

export function fetchShortMetrics() {
  return service.fetchShort()
  .then(short => {
    reactor.dispatch(actionTypes.METRICS_SET_SHORT, short);
  })
}

export function fetchMetrics(){
  return $.when(
    service.fetchLong(),
    service.fetchShort(),
  )
  .done((...responses) => {
    const [ long, short ] = responses;
    reactor.batch(() => {
      reactor.dispatch(actionTypes.METRICS_SET_SHORT, short);
      reactor.dispatch(actionTypes.METRICS_SET_LONG, long);
    });
  })
  .fail(err => {
    logger.error('fetchMetrics', err);
  })
}