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

import Text from 'design/Text';
import Flex from 'design/Flex';

import { Stage } from '../stages';

import { AWSWrapper } from './SharedComponents';

import {
  Content,
  Footer,
  Header,
  NextButton,
  Page,
  RoleButton,
  Section,
  SectionContent,
  SectionDropdown,
  SectionDropdownSelected,
  SectionTitle,
  SubHeader,
  TableCheckBox,
  TableHeader,
  TableItem,
  TableSearch,
  TableTitle,
} from './common';

import type { CommonIAMProps } from './common';

const CreatePolicyButton = styled.div`
  display: flex;
  justify-content: space-between;
`;

const Policies = styled.div`
  margin-top: 20px;
`;

export function IAMCreateNewRolePermissions(props: CommonIAMProps) {
  let content = (
    <>
      <SubHeader>Attach permissions policies</SubHeader>

      <CreatePolicyButton>
        <RoleButton>Create policy</RoleButton>
      </CreatePolicyButton>

      <Footer>
        <NextButton>Next: Tags</NextButton>
      </Footer>
    </>
  );

  if (props.stage >= Stage.RoleReview) {
    content = (
      <>
        <Text fontSize={4} mb={4}>
          Review
        </Text>

        <Section>
          <SectionTitle>Role Name*</SectionTitle>

          <SectionContent>
            <SectionDropdown>
              <SectionDropdownSelected>
                {props.stage >= Stage.RoleHasName ? (
                  'SomeRoleName'
                ) : (
                  <>&nbsp;</>
                )}
              </SectionDropdownSelected>
            </SectionDropdown>
          </SectionContent>
        </Section>

        <Footer>
          <NextButton>Create role</NextButton>
        </Footer>
      </>
    );
  }

  if (
    props.stage >= Stage.RoleTags &&
    props.stage <= Stage.RoleClickNextReview
  ) {
    content = (
      <>
        <Text fontSize={4} bold>
          Add tags - <em>optional</em>
        </Text>

        <Flex mt={4}>
          <RoleButton>Add tag</RoleButton>
        </Flex>

        <Footer>
          <NextButton>Next: Review</NextButton>
        </Footer>
      </>
    );
  }

  if (
    props.stage >= Stage.AssignPolicyToRole &&
    props.stage <= Stage.RoleClickNextTags
  ) {
    content = (
      <>
        <SubHeader>Attach permissions policies</SubHeader>

        <CreatePolicyButton>
          <RoleButton>Create policy</RoleButton>

          <RoleButton>Refresh</RoleButton>
        </CreatePolicyButton>

        <Policies>
          <TableTitle>
            <TableSearch>
              {props.stage >= Stage.SearchForPolicy
                ? 'SomePolicyName'
                : 'Search'}
            </TableSearch>
          </TableTitle>
          {props.stage >= Stage.SearchForPolicy ? (
            <div>
              <TableHeader>Policy Name</TableHeader>
              <TableItem selected={props.stage >= Stage.PolicySelected}>
                <TableCheckBox />
                SomePolicyName
              </TableItem>
            </div>
          ) : (
            <div>
              <TableHeader>Policy Name</TableHeader>
              <TableItem>
                <TableCheckBox />
                AWSDefaultPolicy
              </TableItem>
              <TableItem>
                <TableCheckBox />
                AWSDefaultPolicy2
              </TableItem>
              <TableItem>
                <TableCheckBox />
                AWSDefaultPolicy3
              </TableItem>
              <TableItem>
                <TableCheckBox />
                AWSDefaultPolicy4
              </TableItem>
            </div>
          )}
        </Policies>

        <Footer>
          <NextButton>Next: Tags</NextButton>
        </Footer>
      </>
    );
  }

  return (
    <AWSWrapper>
      <Page>
        <Content>
          <Header>Create role</Header>

          {content}
        </Content>
      </Page>
    </AWSWrapper>
  );
}
