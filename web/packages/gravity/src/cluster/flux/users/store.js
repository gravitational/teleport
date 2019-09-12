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

import { Store, toImmutable } from 'nuclear-js';
import { List, Record } from 'immutable';
import { CLUSTER_RECEIVE_USERS } from './actionTypes';

export class UserRec extends Record({
  userId: '',
  isNew: false,
  name: '',
  email: '',
  status: '',
  builtin: false,
  created: null,
  roles: [],
  owner: false
}) {
  constructor({created, roles, userId, ...props }) {
    created = created || new Date();
    created = new Date(created);
    userId = userId || props.email || props.name;
    roles = roles || [];
    roles = roles.filter(name => !isHiddenRoleName(name));
    super({ created, userId, roles, ...props});
  }
}

export default Store({
  getInitialState() {
    return toImmutable({
      users: [],
    });
  },

  initialize() {
    this.on(CLUSTER_RECEIVE_USERS, receiveUsers);
  }
})

function receiveUsers(state, json) {
  json = json || [];
  let userList = new List( json.map( i => new UserRec(i)) )
  return state.setIn(['users'], userList);
}


function isHiddenRoleName(name) {
  name = name || '';
  return name.indexOf('ca:') === 0;
}