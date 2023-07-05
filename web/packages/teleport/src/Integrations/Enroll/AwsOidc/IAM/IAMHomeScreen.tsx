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

import Text from 'design/Text';
import Flex from 'design/Flex';
import Box from 'design/Box';

import { AWSWrapper } from './SharedComponents';

import {
  Blur,
  Content,
  Page,
  Sidebar,
  SidebarLink,
  SidebarLinkActive,
  SidebarSectionTitle,
  SidebarTitle,
  Title,
} from './common';

export function IAMHomeScreen() {
  return (
    <AWSWrapper>
      <Page>
        <Sidebar>
          <SidebarTitle>Identity and Access Management (IAM)</SidebarTitle>

          <SidebarLinkActive>Dashboard</SidebarLinkActive>

          <SidebarSectionTitle>Access Management</SidebarSectionTitle>

          <SidebarLink>User groups</SidebarLink>
          <SidebarLink>Users</SidebarLink>
          <SidebarLink>Roles</SidebarLink>
          <SidebarLink style={{ background: 'white' }}>
            Identity providers
          </SidebarLink>
          <SidebarLink>Account settings</SidebarLink>

          <SidebarSectionTitle>Access reports</SidebarSectionTitle>

          <SidebarLink>Access analyzer</SidebarLink>
          <SidebarLink>Credential report</SidebarLink>
          <SidebarLink>Organization activity</SidebarLink>
        </Sidebar>
        <Content>
          <Title>IAM Dashboard</Title>

          <Blur>
            <Text mb={3}>Some blurred text here. Some other text here.</Text>
            <Text mb={5}>Some blurred text here</Text>
            <Text>IAM Resources</Text>
            <Flex justifyContent="space-between" mt={3}>
              <Box>
                User groups
                <div style={{ marginTop: 10, fontSize: 28, color: '#1166bb' }}>
                  10
                </div>
              </Box>
              <Box>
                Users
                <div style={{ marginTop: 10, fontSize: 28, color: '#1166bb' }}>
                  145
                </div>
              </Box>
              <Box>
                Identity providers
                <div style={{ marginTop: 10, fontSize: 28, color: '#1166bb' }}>
                  0
                </div>
              </Box>
            </Flex>
          </Blur>
        </Content>
      </Page>
    </AWSWrapper>
  );
}
