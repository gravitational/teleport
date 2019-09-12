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
import { Box } from 'design';
import { useFluxStore } from 'gravity/components/nuclear';
import { withState } from 'shared/hooks';
import { getters as operationGetters } from 'gravity/cluster/flux/operations';
import { fetchOpProgress } from 'gravity/cluster/flux/operations/actions';
import Progress from './Progress';

function OperationBanner(props){
  const { operations, onFetchProgress, ...rest } = props;
  if(operations.length === 0){
    return null;
  }

  // banner needs only 1 operation
  const firstActive = operations[0];
  return (
    <Box {...rest}>
      <Progress
        operation={firstActive}
        onFetch={onFetchProgress}
      />
    </Box>
  )
}

const mapState = () => {
  const opsStore = useFluxStore(operationGetters.operationStore);
  const operations =  opsStore.getActive();
  return {
    onFetchProgress: id => fetchOpProgress(id),
    operations
  }
}

export default withState(mapState)(OperationBanner);