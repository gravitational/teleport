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

import reactor from 'gravity/reactor';
import Logger from 'shared/libs/logger';
import { RECEIVE_OPERATIONS, RECEIVE_PROGRESS  } from './actionTypes';
import opsService from 'gravity/services/operations';

const logger = Logger.create('flux/operations');

export function fetchOps(){
  return opsService.fetchOps().then(ops => {
      reactor.dispatch(RECEIVE_OPERATIONS, ops);
    })
    .fail(err => {
      logger.error('fetchClusterOperations', err);
      throw err;
  })
}

export function fetchOpProgress(opId){
  return opsService.fetchProgress({opId}).then(progress => {
    reactor.dispatch(RECEIVE_PROGRESS, progress);
  })
}
