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
import Certificate from 'gravity/cluster/components/Certificate'
import { fetchTlsCert } from 'gravity/cluster/flux/tlscert/actions';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';

class FeatureCertificate extends FeatureBase {

  constructor(route) {
    super();
    this._route = route;
    this.Component = withFeature(this)(Certificate);
  }

  getRoute(){
    return {
      title: 'Certificate',
      path: cfg.routes.siteCertificate,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    if(!featureFlags.clusterCert()){
      this.setDisabled();
      return;
    }

    addSideNavItem({
      title: 'HTTPS Certificate',
      Icon: Icons.License,
      exact: true,
      to: cfg.getSiteCertificateRoute()
    });

    this.setProcessing();
    return fetchTlsCert()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureCertificate;