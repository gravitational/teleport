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

import * as Icons from 'design/Icon';
import Account from 'teleport/components/Account';
import withFeature, { FeatureBase } from 'teleport/components/withFeature';
import cfg from 'teleport/config';

class AccountFeature extends FeatureBase {
  constructor() {
    super();
    this.Component = withFeature(this)(Account);
  }

  getRoute() {
    return {
      title: 'Account',
      path: cfg.routes.account,
      exact: true,
      component: this.Component,
    };
  }

  onload({ context }) {
    if (!context.isAccountEnabled()) {
      this.setDisabled();
      return;
    }

    context.storeNav.addTopMenuItem({
      title: 'Account Settings',
      Icon: Icons.User,
      to: cfg.routes.account,
    });
  }
}

export default AccountFeature;
