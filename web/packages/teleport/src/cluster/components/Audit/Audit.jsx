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
import { Redirect, Switch, Route } from 'shared/components/Router';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Tabs, { TabItem } from './Tabs';
import AuditSessions from './AuditSessions';
import AuditEvents from './AuditEvents';
import RangePicker, { getRangeOptions } from './RangePicker';
import cfg from 'teleport/config';

export default function Audit() {
  const rangeOptions = React.useMemo(() => getRangeOptions(), []);
  const [range, handleOnRange] = React.useState(rangeOptions[0]);
  const auditRoute = cfg.getAuditRoute();
  const eventsRoute = cfg.getAuditEventsRoute();
  const sessionsRoute = cfg.getAuditSessionsRoute();

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Audit Log</FeatureHeaderTitle>
        <Tabs>
          <TabItem to={eventsRoute} title="Events" />
          <TabItem to={sessionsRoute} title="Sessions" />
        </Tabs>
        <RangePicker
          ml="auto"
          value={range}
          options={rangeOptions}
          onChange={handleOnRange}
        />
      </FeatureHeader>
      <Switch>
        <Route title="System Events" path={eventsRoute}>
          <AuditEvents range={range} />
        </Route>
        <Route
          title="Sessions"
          path={sessionsRoute}
          component={AuditSessions}
        />
        <Redirect exact from={auditRoute} to={eventsRoute} />
      </Switch>
    </FeatureBox>
  );
}
