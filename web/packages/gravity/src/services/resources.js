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

import api from 'gravity/services/api';
import { ResourceEnum } from 'gravity/services/enums';
import cfg from 'gravity/config';

const unpackItems = res => res.items || [];

export function getAuthProviders(){
  return api.get(cfg.getSiteResourcesUrl(ResourceEnum.AUTH_CONNECTORS))
    .then(unpackItems)
}

export function getRoles(){
  return api.get(cfg.getSiteResourcesUrl(ResourceEnum.ROLE))
    .then(unpackItems)
}

export function upsert(kind, yaml, isNew=false){
  const req = { kind, content: yaml };
  if(isNew){
    return api.post(cfg.getSiteResourcesUrl(), req).then(unpackItems)
  }

  return api.put(cfg.getSiteResourcesUrl(), req).then(unpackItems)
}

export function remove(kind, name){
  return api.delete(cfg.getSiteRemoveResourcesUrl(kind, name))
}

export function getForwarders(){
  return api.get(cfg.getSiteResourcesUrl(ResourceEnum.LOG_FWRD))
    .then(unpackItems)
}
