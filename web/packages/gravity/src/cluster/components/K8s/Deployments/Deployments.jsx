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
import { useFluxStore } from 'gravity/components/nuclear';
import { withState } from 'shared/hooks';
import { getters } from 'gravity/cluster/flux/k8s';
import { sortBy } from 'lodash';
import DeploymentList from './DeploymentList';
import { useK8sContext } from './../k8sContext';
import Poller from './../components/Poller';
import { fetchDaemonSets } from 'gravity/cluster/flux/k8s/actions';

export function Deployments(props){
  let { deployments, namespace, onFetch } = props;
  deployments = deployments.filter( item => item.namespace === namespace );
  deployments = sortBy(deployments, ['created']).reverse();
  return (
    <React.Fragment>
      <Poller namespace={namespace} onFetch={onFetch} />
      <DeploymentList
        namespace={namespace}
        deployments={deployments}
      />
    </React.Fragment>
  )
}

export default withState(() => {
  const deployments = useFluxStore(getters.k8sDeployments);
  const { namespace } = useK8sContext();
  return {
    namespace,
    deployments,
    onFetch: fetchDaemonSets,
  }
})(Deployments);
