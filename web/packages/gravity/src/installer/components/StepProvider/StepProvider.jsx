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
import { Danger } from 'design/Alert';
import { Box, ButtonPrimary } from 'design';
import { useValidation } from 'shared/components/Validation';
import { useAttempt } from 'shared/hooks';
import FieldInput from 'shared/components/FieldInput';
import cfg from 'gravity/config';
import history from 'gravity/services/history';
import service from 'gravity/installer/services/installer';
import { StepLayout } from '../Layout';
import { useInstallerContext } from './../store';
import AdvancedOptions, { Subnets } from './AdvancedOptions';

export default function StepProvider() {
  const store = useInstallerContext();
  const { clusterName, serviceSubnet, podSubnet } = store.state;
  const validator = useValidation();
  const [attempt, attemptActions] = useAttempt();
  const { isFailed, isProcessing, message } = attempt;

  function onChangeName(name) {
    store.setClusterName(name);
  }

  function onStart(request) {
    return service.createCluster(request).then(clusterName => {
      history.push(cfg.getInstallerProvisionUrl(clusterName), true);
    });
  }

  function onChangeSubnets({ podSubnet, serviceSubnet }) {
    store.setOnpremSubnets(serviceSubnet, podSubnet);
  }

  function onChangeTags(tags) {
    store.setClusterTags(tags);
  }

  function onContinue() {
    if (validator.validate()) {
      attemptActions.start();
      const request = store.makeOnpremRequest();
      onStart(request).fail(err => attemptActions.error(err));
    }
  }

  return (
    <StepLayout title="Name your cluster">
      <FieldInput
        placeholder="prod.example.com"
        autoFocus
        rule={required}
        value={clusterName}
        onChange={e => onChangeName(e.target.value)}
        label="Cluster Name"
      />
      <Box>
        {isFailed && <Danger mb="4">{message}</Danger>}
        <AdvancedOptions onChangeTags={onChangeTags}>
          <Subnets
            serviceSubnet={serviceSubnet}
            podSubnet={podSubnet}
            onChange={onChangeSubnets}
          />
        </AdvancedOptions>
        <ButtonPrimary
          disabled={isProcessing}
          mt="6"
          width="200px"
          onClick={onContinue}
        >
          Continue
        </ButtonPrimary>
      </Box>
    </StepLayout>
  );
}

const required = value => () => ({
  valid: !!value,
  message: 'Cluster name is required',
});
