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

import { at } from 'lodash';
import { makeStatus } from 'gravity/services/clusters/makeCluster';

export default function makeInfo(json){
  const [
    clusterState,
    gravityUrl,
    internalUrls,
    publicUrls,
    proxyUrl,
    advertiseIp,
  ] = at(json, [
    'clusterState',
    'gravityURL',
    'internalURLs',
    'publicURL',
    // take the first one
    'authGateways[0]',
    // take the first one
    'masterNodes[0]',
  ]);

  const status = makeStatus(clusterState);

  return {
    status,
    gravityUrl,
    internalUrls,
    publicUrls: publicUrls || ['public URL is not set'],
    proxyUrl,
    tshLogin: `tsh login --proxy=${proxyUrl}`,
    advertiseIp,
  }
}