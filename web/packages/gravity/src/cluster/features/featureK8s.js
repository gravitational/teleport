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

import $ from 'jquery';
import cfg from 'gravity/config'
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import * as Icons from 'design/Icon';
import K8s from '../components/K8s';
import {fetchServices} from 'gravity/cluster/flux/k8sServices/actions';
import {fetchNamespaces} from 'gravity/cluster/flux/k8sNamespaces/actions';
import {fetchPods} from 'gravity/cluster/flux/k8sPods/actions';
import {fetchJobs, fetchDaemonSets, fetchDeployments} from 'gravity/cluster/flux/k8s/actions';
import {fetchCfgMaps} from 'gravity/cluster/flux/k8sConfigMaps/actions';

class FeatureK8s extends FeatureBase{

  constructor(){
    super()
    this.Component =  withFeature(this)(K8s);
  }

  getRoute(){
    return {
      path: cfg.routes.siteK8s,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if (!featureFlags.siteK8s()) {
      this.setDisabled();
      return;
    }

    addSideNavItem({
      title: 'Kubernetes',
      Icon: Icons.Kubernetes,
      exact: false,
      to: cfg.getSiteK8sRoute(),
    })

    this.setProcessing();
    $.when(
      fetchDeployments(),
      fetchServices(),
      fetchNamespaces(),
      fetchPods(),
      fetchJobs(),
      fetchCfgMaps(),
      fetchDaemonSets())
      .done(() => this.setReady())
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureK8s;
