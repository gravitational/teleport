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

import { Store, toImmutable } from 'nuclear-js';
import { TLPT_SITES_RECEIVE } from './actionTypes';
import { Record, List } from 'immutable';

const Site = Record({
  name: null,
  status: false
})

export default Store({
  getInitialState() {
    return new List();
  },

  initialize() {
    this.on(TLPT_SITES_RECEIVE, receiveSites)
  }
})

function receiveSites(state, json){    
  return toImmutable(json).map( o => new Site(o) );
}
