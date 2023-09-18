/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Text } from 'design';

import { AwsRegionSelector } from './AwsRegionSelector';

export default {
  title: 'Teleport/Discover/Shared/AwsRegionSelector',
};

export const Disabled = () => (
  <>
    <Text mt={4}>
      Select the AWS Region you would like to see resources for:
    </Text>
    <AwsRegionSelector
      onFetch={() => null}
      onRefresh={() => null}
      disableSelector={true}
      clear={() => null}
    />
  </>
);

export const Enabled = () => (
  <>
    <Text mt={4}>
      Select the AWS Region you would like to see resources for:
    </Text>
    <AwsRegionSelector
      onFetch={() => null}
      onRefresh={() => null}
      disableSelector={false}
      clear={() => null}
    />
  </>
);
