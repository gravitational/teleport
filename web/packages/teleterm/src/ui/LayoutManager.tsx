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
import React, { useRef } from 'react';
import { Flex } from 'design';
/* eslint-disable @typescript-eslint/ban-ts-comment*/
// @ts-ignore
import { AccessRequestCheckout } from 'e-teleterm/ui/AccessRequestCheckout';

import { TabHostContainer } from 'teleterm/ui/TabHost';
import { TopBar } from 'teleterm/ui/TopBar';
import { StatusBar } from 'teleterm/ui/StatusBar';
import { NotificationsHost } from 'teleterm/ui/components/Notifcations';

export function LayoutManager() {
  const topBarContainerRef = useRef<HTMLDivElement>();

  return (
    <Flex flex="1" flexDirection="column" minHeight={0}>
      <TopBar topBarContainerRef={topBarContainerRef} />
      <Flex
        flex="1"
        minHeight={0}
        css={`
          position: relative;
        `}
      >
        <TabHostContainer topBarContainerRef={topBarContainerRef} />
        <NotificationsHost />
      </Flex>
      <AccessRequestCheckout />
      <StatusBar />
    </Flex>
  );
}
