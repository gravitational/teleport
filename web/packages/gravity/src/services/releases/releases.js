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
import cfg from 'gravity/config';
import { map } from 'lodash';
import makeRelease from './makeRelease';

export const service = {
  fetchReleases(siteId){
    return api.get(cfg.getSiteAppsUrl(siteId))
      .then(response => map(response, makeRelease));
  }
}