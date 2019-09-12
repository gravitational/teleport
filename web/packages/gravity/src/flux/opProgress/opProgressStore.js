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

import cfg from 'gravity/config';
import { Store, toImmutable } from 'nuclear-js';
import { OP_PROGRESS_RECEIVE } from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(OP_PROGRESS_RECEIVE, receiveOpProgress);
  }
})

function receiveOpProgress(state, json={}){
  let {site_domain, operation_id } = json;
  let crashReportUrl = cfg.getSiteOperationReportUrl(site_domain, operation_id)
  let siteUrl = cfg.getSiteRoute(site_domain);
  let prgsMap = toImmutable(json);

  prgsMap = prgsMap.set('crashReportUrl', crashReportUrl)
                   .set('siteUrl', siteUrl)
                   .set('site_id', site_domain)

  return state.set(prgsMap.get('operation_id'), prgsMap);
}
