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
import * as Icons from 'design/Icon';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import Monitoring from './../components/Monitoring';

class MonitorFeature extends FeatureBase {
  constructor() {
    super();
    this.Component = withFeature(this)(Monitoring);
  }

  getRoute() {
    return {
      title: 'Monitoring',
      path: cfg.routes.siteMonitor,
      exact: false,
      component: this.Component,
    };
  }

  onload({ featureFlags }) {
    if (!featureFlags.siteMonitoring()) {
      this.setDisabled();
      return;
    }

    addSideNavItem({
      Icon: Icons.Shart,
      to: cfg.getSiteMonitorRoute(),
      title: 'Monitoring',
      exact: false,
    });
  }
}

export default MonitorFeature;
