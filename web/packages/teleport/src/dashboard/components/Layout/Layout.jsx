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
import styled from 'styled-components';
import { Flex, Text } from 'design';

const FeatureHeader = styled(Flex)`
  flex-shrink: 0;
`;

FeatureHeader.defaultProps = {
  my: 4,
};

const FeatureHeaderTitle = props => <Text typography="h2" as="h2" {...props} />;

const FeatureBox = styled(Flex)`
  width: 100%;
  height: 100%;
  ::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;

FeatureBox.defaultProps = {
  px: 6,
  flexDirection: 'column',
};

const AppVerticalSplit = styled.div`
  position: absolute;
  width: 100%;
  height: 100%;
  display: flex;
`;

const AppHorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
`;

export {
  AppHorizontalSplit,
  AppVerticalSplit,
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
};
