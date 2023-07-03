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
import styled from 'styled-components';

import Flex from 'design/Flex';

import { Stage } from '../stages';

import { AWSWrapper, BreadcrumbArrow } from './SharedComponents';

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbItemActive,
  Content,
  NextButton,
  Page,
  Sidebar,
  SidebarLink,
  SidebarLinkActive,
  SidebarSectionTitle,
  SidebarTitle,
  Title,
} from './common';

import type { CommonIAMProps } from './common';

const Providers = styled.div`
  display: flex;
  margin-top: 20px;
  width: calc(100% - 60px);
`;

const ProviderSection = styled.div`
  flex: 1;
`;

const ProviderSectionTitle = styled.div`
  background: linear-gradient(#eee, #e0e0e0);
  padding: 5px 10px;
  font-weight: 700;
`;

const ProviderTitle = styled.div`
  padding: 5px 10px;
  color: #16b;
`;

const ProviderSectionContent = styled.div`
  padding: 5px 10px;
`;

export function IAMIdentityProvidersScreen(props: CommonIAMProps) {
  if (props.stage >= Stage.ProviderAdded) {
    return (
      <AWSWrapper>
        <Page>
          <Content>
            <Breadcrumb>
              <BreadcrumbItem>IAM</BreadcrumbItem>
              <BreadcrumbArrow />
              <BreadcrumbItemActive>Identity providers</BreadcrumbItemActive>
            </Breadcrumb>

            <Flex mt={5} alignItems="center">
              <Title style={{ marginBottom: 0 }}>Identity Providers (1)</Title>
            </Flex>

            <Providers>
              <ProviderSection>
                <ProviderSectionTitle>Provider</ProviderSectionTitle>

                <ProviderTitle>{props.clusterPublicUri}</ProviderTitle>
              </ProviderSection>
              <ProviderSection>
                <ProviderSectionTitle>Type</ProviderSectionTitle>

                <ProviderSectionContent>OpenID Connect</ProviderSectionContent>
              </ProviderSection>
              <ProviderSection>
                <ProviderSectionTitle>Creation time</ProviderSectionTitle>

                <ProviderSectionContent>1 minute ago</ProviderSectionContent>
              </ProviderSection>
            </Providers>
          </Content>
        </Page>
      </AWSWrapper>
    );
  }

  return (
    <AWSWrapper>
      <Page>
        <Sidebar>
          <SidebarTitle>Identity and Access Management (IAM)</SidebarTitle>

          <SidebarLink>Dashboard</SidebarLink>

          <SidebarSectionTitle>Access Management</SidebarSectionTitle>

          <SidebarLink>User groups</SidebarLink>
          <SidebarLink>Users</SidebarLink>
          <SidebarLink>Roles</SidebarLink>
          <SidebarLinkActive>Identity providers</SidebarLinkActive>
          <SidebarLink>Account settings</SidebarLink>

          <SidebarSectionTitle>Access reports</SidebarSectionTitle>

          <SidebarLink>Access analyzer</SidebarLink>
          <SidebarLink>Credential report</SidebarLink>
          <SidebarLink>Organization activity</SidebarLink>
        </Sidebar>
        <Content>
          <Breadcrumb>
            <BreadcrumbItem>IAM</BreadcrumbItem>
            <BreadcrumbArrow />
            <BreadcrumbItemActive>Identity providers</BreadcrumbItemActive>
          </Breadcrumb>

          <Flex mt={5} alignItems="center">
            <Title style={{ marginBottom: 0 }}>Identity Providers (0)</Title>

            <NextButton>Add provider</NextButton>
          </Flex>
        </Content>
      </Page>
    </AWSWrapper>
  );
}
