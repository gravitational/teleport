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
import { useK8sContext } from './../k8sContext';
import DaemonSetList from './DaemonSetList';
import Poller from './../components/Poller';
import { getters } from 'gravity/cluster/flux/k8s';
import { fetchDaemonSets } from 'gravity/cluster/flux/k8s/actions';

export function DaemonSets(props) {
  const { daemonSets, namespace, onFetch } = props;
  return (
    <React.Fragment>
      <Poller namespace={namespace} onFetch={onFetch} />
      <DaemonSetList
        namespace={namespace}
        daemonSets={daemonSets}
      />
    </React.Fragment>
  )
}

export default withState(() => {
  const daemonSets = useFluxStore(getters.k8sDaemonSets);
  const { namespace } = useK8sContext();
  return {
    namespace,
    daemonSets,
    onFetch: fetchDaemonSets,
  }
})(DaemonSets);
