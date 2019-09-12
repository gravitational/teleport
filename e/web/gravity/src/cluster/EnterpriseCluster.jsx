import React from 'react';

// oss imports
import Cluster from 'gravity/cluster/components';
import { initCluster } from 'gravity/cluster/flux/actions';
import FeatureDashboard from 'gravity/cluster/features/featureDashboard';
import FeatureAccount from 'gravity/cluster/features/featureAccount';
import FeatureNodes from 'gravity/cluster/features/featureNodes';
import FeatureLogs from 'gravity/cluster/features/featureLogs';
import FeatureMonitoring from 'gravity/cluster/features/featureMonitoring';
import FeatureCertificate from 'gravity/cluster/features/featureCertificate';
import FeatureAudit from 'gravity/cluster/features/featureAudit';
import FeatureK8s from 'gravity/cluster/features/featureK8s';
import 'gravity/cluster/flux';

import { withState } from 'shared/hooks';
import FeatureLicense from './features/featureLicense';
import FeatureRoles from './features/featureRoles';
import FeatureAuthConnectors from './features/featureAuthConnectors';
import FeatureUsers from './features/featureUsers';
import './flux';

function mapState(props){
  const { siteId } = props.match.params;
  const [ features ] = React.useState(() => {
    return [
      new FeatureDashboard(),
      new FeatureAccount(),
      new FeatureNodes(),
      new FeatureLogs(),
      new FeatureAudit(),
      new FeatureRoles(),
      new FeatureUsers(),
      new FeatureK8s(),
      new FeatureMonitoring(),
      new FeatureAuthConnectors(),
      new FeatureCertificate(),
      new FeatureLicense(),
    ]
  })

  function onInit(){
    return initCluster(siteId, features);
  }

  return {
    features,
    onInit,
  }
}

export default withState(mapState)(Cluster);