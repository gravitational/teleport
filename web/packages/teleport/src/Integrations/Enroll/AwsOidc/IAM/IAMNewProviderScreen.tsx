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

import Box from 'design/Box';
import Flex from 'design/Flex';

import { Stage } from '../stages';

import { AWSWrapper, BreadcrumbArrow } from './SharedComponents';

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbItemActive,
  Content,
  Page,
  Sidebar,
  SidebarLink,
  SidebarLinkActive,
  SidebarSectionTitle,
  SidebarTitle,
  Title,
} from './common';

import { OpenIDForm } from './OpenIDForm';

import type { CommonIAMProps } from './common';

const ProviderType = styled.div<{ active: boolean }>`
  flex: 0 0 220px;
  border: 1px solid ${p => (p.active ? '#3388dd' : '#cccccc')};
  background: ${p => (p.active ? '#f0f9ff' : 'white')};
  border-radius: 5px;
  display: flex;
  margin-right: 10px;
`;

const ProviderTypeIconContainer = styled.div`
  flex: 0 0 30px;
  display: flex;
  justify-content: center;
  padding-top: 12px;
`;

const ProviderTypeIcon = styled.div`
  width: 10px;
  height: 10px;
  background: ${p => (p.active ? '#1066bb' : 'white')};
  border: 1px solid ${p => (p.active ? '#1066bb' : '#cccccc')};
  border-radius: 50%;
  position: relative;

  &:after {
    content: '';
    visibility: ${p => (p.active ? 'visible' : 'hidden')};
    position: absolute;
    width: 5px;
    height: 5px;
    background: white;
    border-radius: 50%;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
  }
`;

const ProviderTypeInfo = styled.div`
  padding: 10px 0;
  font-size: 10px;
  color: #999;
  line-height: 12px;
`;

const ProviderTypeTitle = styled.div`
  font-size: 14px;
  line-height: 16px;
  margin-bottom: 10px;
  color: #444;
`;

const SlideOutPage = styled(Page)`
  transition: 1s transform cubic-bezier(0.4, 0, 0.2, 1);
  transform: translate3d(${p => (p.hidden ? -251 : 0)}px, 0, 0);
`;

export function IAMNewProviderScreen(props: CommonIAMProps) {
  const sidebarHidden = props.stage !== Stage.NewProvider;
  const openIDSelected = props.stage >= Stage.OpenIDConnectSelected;

  return (
    <AWSWrapper>
      <SlideOutPage hidden={sidebarHidden}>
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
            <BreadcrumbItem>Identity providers</BreadcrumbItem>
            <BreadcrumbArrow />
            <BreadcrumbItemActive>Create</BreadcrumbItemActive>
          </Breadcrumb>

          <Box mt={5}>
            <Title>Add an Identity Provider </Title>
          </Box>

          <Flex>
            <ProviderType active={!openIDSelected}>
              <ProviderTypeIconContainer>
                <ProviderTypeIcon active={!openIDSelected} />
              </ProviderTypeIconContainer>

              <ProviderTypeInfo>
                <ProviderTypeTitle>SAML</ProviderTypeTitle>
                Establish trust between your AWS account and a SAML 2.0
                compatible Identity Provider such as Shibboleth or Active
                Directory Federation Services.
              </ProviderTypeInfo>
            </ProviderType>

            <ProviderType active={openIDSelected}>
              <ProviderTypeIconContainer>
                <ProviderTypeIcon active={openIDSelected} />
              </ProviderTypeIconContainer>

              <ProviderTypeInfo>
                <ProviderTypeTitle>OpenID Connect</ProviderTypeTitle>
                Establish trust between your AWS account and Identity Provider
                services, such as Google or Salesforce.
              </ProviderTypeInfo>
            </ProviderType>
          </Flex>

          {openIDSelected && (
            <OpenIDForm
              stage={props.stage}
              clusterPublicUri={props.clusterPublicUri}
            />
          )}
        </Content>
      </SlideOutPage>
    </AWSWrapper>
  );
}
