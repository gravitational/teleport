// oss imports
import FeatureUsers, { makeNavItem } from 'gravity/cluster/features/featureUsers';
import withFeature from 'gravity/components/withFeature';
import EnterpiseUsers from 'e-gravity/cluster/components/Users';

class EnterpriseUsersFeature extends FeatureUsers {
  constructor() {
    super()
    this.Component = withFeature(this)(EnterpiseUsers);
  }
}

export default EnterpriseUsersFeature;

export {
  makeNavItem
}