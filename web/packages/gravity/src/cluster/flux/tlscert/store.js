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

import { Record } from 'immutable';
import { Store, toImmutable } from 'nuclear-js';
import { SETTINGS_CERT_RECEIVE } from './actionTypes';

export class TlsCert extends Record({
  issued_by: null,
  issued_to: null,
  validity: null,

}) {
  constructor(json) {
    super(toImmutable(json));
  }

  getByCn() {
    return this.getIn(['issued_by', 'cn']);
  }

  getByOrg() {
    let org = this.getIn(['issued_by', 'org']);
    org = org ? org.toJS() : [];
    return org.join(', ');
  }

  getByOrgUnit() {
    return this.getIn(['issued_by', 'org_unit']);
  }

  getToCn() {
    return this.getIn(['issued_to', 'cn']);
  }

  getToOrgUnit() {
    return this.getIn(['issued_to', 'org_unit']);
  }

  getToOrg() {
    let org = this.getIn(['issued_to', 'org']);
    org = org ? org.toJS() : [];
    return org.join(', ');
  }

  getStartDate() {
    let date = this.getIn(['validity', 'not_before']);
    return new Date(date).toUTCString();
  }

  getEndDate() {
    let date = this.getIn(['validity', 'not_after']);
    return new Date(date).toUTCString();
  }

}

export default Store({
  getInitialState() {
    return new TlsCert();
  },

  initialize() {
    this.on(SETTINGS_CERT_RECEIVE, setTlsCert);
  }
})

function setTlsCert(state, json) {
  return new TlsCert(json);
}