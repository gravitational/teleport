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

import { Content, Page } from './common';

import type { CommonIAMProps } from './common';

const RolesSuccess = styled.div`
  background: #019934;
  color: white;
  padding: 10px 15px;
  font-size: 16px;
`;

const RolesSuccessLink = styled.span`
  text-decoration: underline;
`;

const RolesHeader = styled.div`
  background-image: linear-gradient(#fff, #eee);
  padding: 20px 20px;
  border-top: 1px solid #ccc;
  border-bottom: 1px solid #ccc;
  font-weight: bold;
  font-size: 22px;
`;

const PoliciesTableHeader = styled.div`
  background-image: linear-gradient(#eee, #e0e0e0);
  padding: 10px 45px;
  font-weight: bold;
  border: 1px solid #ccc;
  border-top: none;
`;

const PoliciesCheckBox = styled.div`
  width: 10px;
  height: 10px;
  margin-right: 20px;
  border-radius: 3px;
`;

const PoliciesTableItem = styled.div<{ selected?: boolean }>`
  display: flex;
  align-items: center;
  padding: 10px 15px;
  border-bottom: 1px solid #cccccc;
  background: ${p => (p.selected ? '#e6f3ff' : 'white')};

  ${PoliciesCheckBox} {
    background: ${p => (p.selected ? '#1066bb' : 'white')};
    border: 1px solid ${p => (p.selected ? '#1066bb' : '#cccccc')};
  }
`;

const RoleName = styled.div`
  font-size: 24px;
`;

const SummaryTitle = styled.div`
  font-weight: bold;
  font-size: 18px;
  margin-top: 30px;
`;

const Summary = styled.div`
  margin-top: 20px;
  border-top: 1px solid #ccc;
  display: flex;
`;

const SummarySection = styled.div`
  margin-top: 5px;
  flex: 1;
  padding-top: 10px;
`;

const SectionLabel = styled.div`
  font-size: 14px;
  color: rgba(0, 0, 0, 0.6);
`;

const SectionValue = styled.div`
  font-size: 16px;
`;

export function IAMRoles(props: CommonIAMProps) {
  let content;
  if (props.stage >= Stage.ListRoles && props.stage <= Stage.ClickRole) {
    content = (
      <div>
        <RolesSuccess>
          The role <RolesSuccessLink>SomeRoleName</RolesSuccessLink> has been
          created
        </RolesSuccess>
        <Content>
          <div>
            <RolesHeader>Roles</RolesHeader>

            <div>
              <PoliciesTableHeader>Role Name</PoliciesTableHeader>
              <PoliciesTableItem>
                <PoliciesCheckBox />
                AWSDefaultRole
              </PoliciesTableItem>
              <PoliciesTableItem>
                <PoliciesCheckBox />
                AWSDefaultRole2
              </PoliciesTableItem>
              <PoliciesTableItem>
                <PoliciesCheckBox />
                AWSDefaultRole3
              </PoliciesTableItem>
              <PoliciesTableItem>
                <PoliciesCheckBox />
                AWSDefaultRole4
              </PoliciesTableItem>
            </div>
          </div>
        </Content>
      </div>
    );
  }

  if (props.stage >= Stage.ViewRole) {
    content = (
      <Content>
        <RoleName>SomeRoleName</RoleName>

        <SummaryTitle>Summary</SummaryTitle>

        <Summary>
          <SummarySection style={{ borderRight: '1px solid #ccc' }}>
            <SectionLabel>Creation Date</SectionLabel>
            <SectionValue>Just now</SectionValue>

            <SectionLabel>Lst activity</SectionLabel>
            <SectionValue>None</SectionValue>
          </SummarySection>
          <SummarySection style={{ paddingLeft: '20px' }}>
            <SectionLabel>ARN</SectionLabel>
            <SectionValue>
              arn:aws:iam::123456789:role/SomeRoleName
            </SectionValue>
          </SummarySection>
        </Summary>
      </Content>
    );
  }

  return (
    <AWSWrapper>
      <Page>{content}</Page>
    </AWSWrapper>
  );
}
