/*
Copyright 2021 Gravitational, Inc.

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

import ConnectDialog from './ConnectDialog';

export default {
  title: 'Teleport/Databases/Connect',
};

export const Connect = () => (
  <ConnectDialog
    username="yassine"
    dbName="aurora"
    dbProtocol="postgres"
    clusterId="im-a-cluster"
    onClose={() => null}
    authType="local"
  />
);

export const ConnectWithRequestId = () => {
  return (
    <ConnectDialog
      username="yassine"
      dbName="aurora"
      dbProtocol="postgres"
      clusterId="im-a-cluster"
      onClose={() => null}
      authType="local"
      accessRequestId="e1e8072c-1eb8-5df4-a7bd-b6863b19271c"
    />
  );
};
