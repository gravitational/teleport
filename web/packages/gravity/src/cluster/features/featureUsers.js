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
import Users from './../components/Users';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import { fetchUsers } from 'gravity/cluster/flux/users/actions';
import * as Icons from 'design/Icon';

export function makeNavItem(to){
  return {
    title: 'Users',
    Icon: Icons.Users,
    exact: true,
    to
  }
}

class FeatureUsers extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(Users);
  }

  getRoute(){
    return {
      title: 'Users',
      path: cfg.routes.siteUsers,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if (!featureFlags.siteUsers()) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getSiteUsersRoute());
    addSideNavItem(navItem);

    this.setProcessing();
    fetchUsers()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureUsers;
