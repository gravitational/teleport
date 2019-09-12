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

import React, { useState } from 'react';
import { withState } from 'shared/hooks';
import { Box } from 'design';
import { useFluxStore } from 'gravity/components/nuclear';
import * as featureFlags from 'gravity/cluster/featureFlags';
import { fetchPods } from 'gravity/cluster/flux/k8sPods/actions';
import { getters as aclGetters } from 'gravity/flux/userAcl';
import InputSearch from 'gravity/cluster/components/components/InputSearch';
import { getters } from 'gravity/cluster/flux/k8sPods';
import { useK8sContext } from './../k8sContext';
import Poller from './../components/Poller';
import PodList from './PodList/PodList';

export function Pods(props) {
  const { namespace, logsEnabled, monitoringEnabled, podInfos, userAcl, onFetch } = props;
  const [searchValue, onSearchChange] = useState('');
  const sshLogins = userAcl.getSshLogins();

  return (
    <React.Fragment>
      <Poller namespace={namespace} onFetch={onFetch} />
      <Box bg="primary.light" p="3" borderTopLeftRadius="3" borderTopRightRadius="3">
        <InputSearch autoFocus onChange={onSearchChange} />
      </Box>
      <PodList
        logsEnabled={logsEnabled}
        monitoringEnabled={monitoringEnabled}
        podInfos={podInfos}
        searchValue={searchValue}
        namespace={namespace}
        sshLogins={sshLogins}
      />
    </React.Fragment>
  );
}

export default withState(() => {
  const { namespace } = useK8sContext();
  const podInfos = useFluxStore(getters.podInfoList);
  const userAcl = useFluxStore(aclGetters.userAcl);
  const monitoringEnabled = featureFlags.siteMonitoring();
  const logsEnabled = featureFlags.siteLogs();
  return {
    monitoringEnabled,
    logsEnabled,
    userAcl,
    namespace,
    podInfos,
    onFetch: fetchPods,
  };
})(Pods);
