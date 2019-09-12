import * as Icons from 'design/Icon';
import Account from 'gravity/cluster/components/Account';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSettingNavItem } from 'e-gravity/hub/flux/nav/actions';
import cfg from 'e-gravity/config'

class FeatureAccount extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(Account);
  }

  getRoute(){
    return {
      title: 'Account',
      path: cfg.routes.hubSettingAccount,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if(!featureFlags.siteAccount()){
      this.setDisabled();
      return;
    }

    addSettingNavItem({
      title: 'Account Settings',
      Icon: Icons.User,
      to: cfg.routes.hubSettingAccount
    });
  }

}

export default FeatureAccount;