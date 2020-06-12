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
import { Route, Switch } from 'teleport/components/Router';
import Console from 'teleport/console';
import Dashboard from 'teleport/dashboard';
import Player from 'teleport/player';
import Teleport from 'teleport/Teleport';
import cfg from 'teleport/config';
import { LicenseEnforcer } from './components/License';
import EnterpriseCluster from './cluster';

export default function TeleportE({ history }) {
  return (
    <Teleport history={history}>
      <LicenseEnforcer />
      <Switch>
        <Route path={cfg.routes.console} component={Console} />
        <Route path={cfg.routes.player} component={Player} />
        <Route path={cfg.routes.cluster} component={EnterpriseCluster} />
        <Route path={cfg.routes.app} component={Dashboard} />
      </Switch>
    </Teleport>
  );
}
