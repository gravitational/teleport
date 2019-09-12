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

import cfg from 'gravity/config'
import * as Icons from 'design/Icon';
import Audit from 'gravity/cluster/components/Audit';
import { fetchLatest } from 'gravity/cluster/flux/events/actions';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';

class FeatureAudit extends FeatureBase {
  constructor() {
    super();
    this.Component = withFeature(this)(Audit);
  }

  getRoute(){
    return {
      title: 'Audit',
      path: cfg.routes.siteAudit,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    if(!featureFlags.clusterEvents()){
      this.setDisabled();
      return;
    }

    addSideNavItem({
      title: 'Audit Log',
      Icon: Icons.ListBullet,
      exact: true,
      to: cfg.getSiteAuditRoute()
    });

    this.setProcessing();
    return fetchLatest()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureAudit;