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

// oss imports
import Cluster from 'gravity/cluster/components';
import { initCluster } from 'gravity/cluster/flux/actions';
import FeatureDashboard from 'gravity/cluster/features/featureDashboard';
import FeatureAccount from 'gravity/cluster/features/featureAccount';
import FeatureNodes from 'gravity/cluster/features/featureNodes';
import FeatureLogs from 'gravity/cluster/features/featureLogs';
import FeatureUsers from 'gravity/cluster/features/featureUsers';
import FeatureMonitoring from 'gravity/cluster/features/featureMonitoring';
import FeatureCertificate from 'gravity/cluster/features/featureCertificate';
import FeatureAudit from 'gravity/cluster/features/featureAudit';
import FeatureK8s from 'gravity/cluster/features/featureK8s';
import { withState } from 'shared/hooks';
import './flux';

function mapState(props) {
  const { siteId } = props.match.params;
  const [features] = React.useState(() => {
    return [
      new FeatureDashboard(),
      new FeatureAccount(),
      new FeatureNodes(),
      new FeatureLogs(),
      new FeatureAudit(),
      new FeatureUsers(),
      new FeatureK8s(),
      new FeatureMonitoring(),
      new FeatureCertificate(),
    ];
  });

  function onInit() {
    return initCluster(siteId, features);
  }

  return {
    features,
    onInit,
  };
}

export default withState(mapState)(Cluster);
