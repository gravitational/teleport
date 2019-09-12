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
import api from 'gravity/services/api';
import cfg from 'gravity/config';
import { UserStatusEnum } from 'gravity/services/enums'
import Logger from 'shared/libs/logger';
import { CLUSTER_RECEIVE_USERS } from './actionTypes';

const logger = Logger.create('cluster/flux/users/actions');

export function createInvite(name, roles){
  const data = { name, roles };
  return api.post(cfg.getSiteUserInvitePath(), data)
    .then(userToken => {
      fetchUsers();
      return userToken;
    })
    .fail(err => {
      logger.error('createInvite()', err);
    })
}

export function resetUser(userId) {
  return api.post(cfg.getSiteUserResetPath({userId}))
    .done(userToken => {
      fetchUsers();
      return userToken;
    })
    .fail(err => {
      logger.error('resetUser()', err);
  })
}

export function saveUser(userId, roles) {
  const data = { email: userId, roles };
  return api.put(cfg.getSiteUserUrl(), data)
    .done(inviteLink => {
      fetchUsers();
      return inviteLink;
    })
    .fail(err => {
      logger.error('saveUser()', err);
    });
}

export function deleteUser(userRec) {
  const { userId } = userRec;
  const isInvite = userRec.status === UserStatusEnum.INVITED;
  const url = isInvite ?
    cfg.getAccountDeleteInviteUrl({ inviteId: userId }) : cfg.getAccountDeleteUserUrl({ userId });

  return api.delete(url)
    .done(()=> {
      fetchUsers();
    })
    .fail(err=> {
      logger.error('deleteUser()', err);
    });
}

export function fetchUsers() {
  return api.get(cfg.getSiteUserUrl()).done(users => {
    reactor.dispatch(CLUSTER_RECEIVE_USERS, users);
  })
}