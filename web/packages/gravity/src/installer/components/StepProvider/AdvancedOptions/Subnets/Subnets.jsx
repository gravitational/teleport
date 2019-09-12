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
import { parseCidr } from 'gravity/lib/paramUtils';
import { Flex } from 'design';
import FieldInput from 'shared/components/FieldInput';

const POD_HOST_NUM = 65534;
const INVALID_SUBNET = 'Invalid CIDR format';
const VALIDATION_POD_SUBNET_MIN = `Range cannot be less than ${POD_HOST_NUM}`;

export default function Subnets({
  onChange,
  podSubnet,
  serviceSubnet,
  ...styles
}) {
  function onChangePodnet(e) {
    onChange({ podSubnet: e.target.value, serviceSubnet });
  }

  function onChangeServiceSubnet(e) {
    onChange({ podSubnet, serviceSubnet: e.target.value });
  }

  return (
    <Flex {...styles}>
      <FieldInput
        flex="1"
        autoComplete="off"
        label="Service Subnet"
        mr="3"
        onChange={onChangeServiceSubnet}
        placeholder="10.0.0.0/16"
        rule={validCidr}
        value={serviceSubnet}
      />
      <FieldInput
        flex="1"
        autoComplete="off"
        label="Pod Subnet"
        onChange={onChangePodnet}
        placeholder="10.0.0.0/16"
        rule={validPod}
        value={podSubnet}
      />
    </Flex>
  );
}

const validCidr = value => () => {
  return {
    valid: parseCidr(value) !== null,
    message: INVALID_SUBNET,
  };
};

const validPod = value => () => {
  const result = parseCidr(value);
  if (result && result.totalHost <= POD_HOST_NUM) {
    return {
      valid: false,
      message: VALIDATION_POD_SUBNET_MIN,
    };
  }

  return {
    valid: result !== null,
    message: INVALID_SUBNET,
  };
};
