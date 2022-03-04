/*
Copyright 2022 Gravitational, Inc.

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
import { Box, Flex } from 'design';
import QuickInput from 'teleterm/ui/QuickInput';
import SplitPane from 'shared/components/SplitPane';
import { Navigator } from 'teleterm/ui/Navigator';
import { TabHostContainer } from 'teleterm/ui/TabHost';
import styled from 'styled-components';
import { Identity } from 'teleterm/ui/Identity/Identity';

export function LayoutManager() {
  return (
    <SplitPane defaultSize='20%' flex="1" split="vertical">
      <Box flex="1" bg="primary.light" width="100%">
        <Navigator />
      </Box>
      <RightPaneContainer flexDirection="column">
        <Flex justifyContent="space-between" p="0 25px">
          <QuickInput />
          <Identity />
        </Flex>
        <Box flex="1" style={{ position: 'relative' }}>
          <TabHostContainer />
        </Box>
      </RightPaneContainer>
    </SplitPane>
  );
}

const RightPaneContainer = styled(Flex)`
  width: 100%;
  flex-direction: column;
`;
