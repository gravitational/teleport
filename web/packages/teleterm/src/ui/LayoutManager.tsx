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

import React, { useEffect, useRef } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { Box, Flex } from 'design';
import { debounce } from 'lodash';
import QuickInput from 'teleterm/ui/QuickInput';
import SplitPane from 'shared/components/SplitPane';
import { Navigator } from 'teleterm/ui/Navigator';
import { TabHost } from 'teleterm/ui/TabHost';
import styled from 'styled-components';

export function LayoutManager() {
  const ctx = useAppContext();
  const navigatorRef = useRef<HTMLDivElement>();
  const defaultNavigatorWidth = ctx.workspaceService.getNavigatorWidth()
    ? `${ctx.workspaceService.getNavigatorWidth()}px`
    : '20%';

  useEffect(() => {
    const updateNavigatorWidth = debounce((width: number) => {
      if (ctx.workspaceService.getNavigatorWidth() !== width) {
        ctx.workspaceService.saveNavigatorWidth(width);
      }
    }, 1000);

    const resizeObserver = new ResizeObserver(entries => {
      updateNavigatorWidth(entries[0].contentRect.width);
    });

    resizeObserver.observe(navigatorRef.current);

    return () => {
      resizeObserver.unobserve(navigatorRef.current);
      updateNavigatorWidth.cancel();
    };
  }, []);

  return (
    <SplitPane defaultSize={defaultNavigatorWidth} flex="1" split="vertical">
      <Box flex="1" bg="primary.light" ref={navigatorRef} width="100%">
        <Navigator />
      </Box>
      <RightPaneContainer flexDirection="column">
        <QuickInput />
        <Box flex="1" style={{ position: 'relative' }}>
          <TabHost />
        </Box>
      </RightPaneContainer>
    </SplitPane>
  );
}

const RightPaneContainer = styled(Flex)`
  width: 100%;
  flex-direction: column;
`;
