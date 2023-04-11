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
