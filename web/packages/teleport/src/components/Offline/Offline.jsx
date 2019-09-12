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

import React from 'react';
import { withRouter } from 'react-router';
import { Offline } from 'design/CardError';

export function OfflineCluster({ match }) {
  const { siteId } = match.params;
  return (
    <Offline
      title="This cluster is not available from Gravity"
      message={`To access "${siteId}" please use its local endpoint.`}
    />
  );
}

export default withRouter(OfflineCluster);
