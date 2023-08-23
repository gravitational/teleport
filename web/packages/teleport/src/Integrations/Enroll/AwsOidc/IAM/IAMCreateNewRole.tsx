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

import { Stage } from '../stages';

import { AWSWrapper } from './SharedComponents';

import {
  Content,
  Footer,
  Header,
  NextButton,
  Page,
  Section,
  SectionContent,
  SectionDropdown,
  SectionDropdownSelected,
  SectionTitle,
  SubHeader,
} from './common';

import type { CommonIAMProps } from './common';

const TrustedEntities = styled.div`
  display: flex;
  margin-bottom: 30px;
`;

const TrustedEntity = styled.div<{ active: boolean }>`
  background: ${p => (p.active ? '#e6f3ff' : 'white')};
  border: 2px solid ${p => (p.active ? '#1066bb' : '#cccccc')};
  flex: 0 0 180px;
  margin-right: 10px;
  display: flex;
`;

const TrustedEntityContent = styled.div`
  padding: 10px 15px;
`;

const TrustedEntityTitle = styled.div`
  font-weight: bold;
`;

const TrustedEntityDescription = styled.div`
  font-size: 12px;
  color: #666;
  line-height: 1.2;
`;

const SectionDropdownItems = styled.div`
  background: #222222;
  position: absolute;
  top: 34px;
  color: white;
  width: 300px;
  border-radius: 4px;
`;

const SectionDropdownItem = styled.div<{ hovered: boolean }>`
  padding: 5px 15px;
  color: ${p => (p.hovered ? '#d27106' : 'white')};
`;

export function IAMCreateNewRole(props: CommonIAMProps) {
  return (
    <AWSWrapper>
      <Page>
        <Content>
          <Header>Create role</Header>

          <SubHeader>Select type of trusted entity</SubHeader>

          <TrustedEntities>
            <TrustedEntity>
              <TrustedEntityContent>
                <TrustedEntityTitle>AWS service</TrustedEntityTitle>

                <TrustedEntityDescription>
                  EC2, Lambda and others
                </TrustedEntityDescription>
              </TrustedEntityContent>
            </TrustedEntity>

            <TrustedEntity active={true}>
              <TrustedEntityContent>
                <TrustedEntityTitle>Web identity</TrustedEntityTitle>

                <TrustedEntityDescription>
                  Cognito or any OpenID provider
                </TrustedEntityDescription>
              </TrustedEntityContent>
            </TrustedEntity>

            <TrustedEntity>
              <TrustedEntityContent>
                <TrustedEntityTitle>SAML 2.0 federation</TrustedEntityTitle>

                <TrustedEntityDescription>
                  Your corporate directory
                </TrustedEntityDescription>
              </TrustedEntityContent>
            </TrustedEntity>
          </TrustedEntities>

          <SubHeader>Choose a web identity provider</SubHeader>

          <Section>
            <SectionTitle>Identity Provider</SectionTitle>

            <SectionContent>
              <SectionDropdown>
                <SectionDropdownSelected>
                  {props.clusterPublicUri}:aud
                </SectionDropdownSelected>
              </SectionDropdown>
            </SectionContent>
          </Section>

          <Section>
            <SectionTitle>Audience*</SectionTitle>

            <SectionContent>
              <SectionDropdown>
                <SectionDropdownSelected>
                  {props.stage >= Stage.DiscoverAudienceSelected ? (
                    'discover.teleport'
                  ) : (
                    <>&nbsp;</>
                  )}
                </SectionDropdownSelected>

                {props.stage >= Stage.ShowAudienceDropdown &&
                  props.stage < Stage.DiscoverAudienceSelected && (
                    <SectionDropdownItems>
                      <SectionDropdownItem>
                        discover.teleport
                      </SectionDropdownItem>
                    </SectionDropdownItems>
                  )}
              </SectionDropdown>
            </SectionContent>
          </Section>

          <Footer>
            <NextButton>Next: Permissions</NextButton>
          </Footer>
        </Content>
      </Page>
    </AWSWrapper>
  );
}
