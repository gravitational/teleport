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
import Text from 'design/Text';

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
} from './common';

import type { CommonIAMProps } from './common';

const Tabs = styled.div`
  display: flex;
  border-bottom: 1px solid #ccc;
`;

const Tab = styled.div<{ active: boolean }>`
  background: ${p => (p.active ? 'white' : '#eeeeee')};
  font-weight: bold;
  border: 1px solid #cccccc;
  border-top-left-radius: 5px;
  border-top-right-radius: 5px;
  padding: 5px 15px;
  margin-right: 10px;
  position: relative;
  overflow: hidden;
  border-top-width: ${p => (p.active ? 0 : '1px')};
  border-bottom-color: ${p => (p.active ? 'white' : '#cccccc')};
  margin-bottom: -1px;

  &:after {
    height: 3px;
    position: absolute;
    left: 0;
    top: 0;
    right: 0;
    background: #e07701;
    content: '';
    display: ${p => (p.active ? 'block' : 'none')};
  }
`;

const JSONEditor = styled.div<{ selected: boolean }>`
  border: 1px solid #ccc;
  margin-top: 20px;
  padding: 0 25px;
  font-size: 14px;
  position: relative;

  &:after {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    content: '';
    background: #b5d5ff;
    z-index: 0;
    display: ${p => (p.selected ? 'block' : 'none')};
  }

  pre {
    position: relative;
    z-index: 1;
  }
`;

export function IAMCreateNewPolicy(props: CommonIAMProps) {
  let jsonEditor;
  if (props.stage >= Stage.ShowJSONEditor) {
    let content = `{
    "Version": "2012-10-17",
    "Statement": []
}`;
    if (props.stage >= Stage.PolicyJSONPasted) {
      content = `{
    "Version": "2012-10-17",
    "Statement": [
        {
          "Effect": "Allow",
          "Action": [
              "rds:DescribeDBInstances",
              "rds:DescribeDBClusters"
          ],
          "Resource": "*"
      }
    ]
}`;
    }

    jsonEditor = (
      <JSONEditor selected={props.stage === Stage.JSONContentsSelected}>
        <pre>{content}</pre>
      </JSONEditor>
    );
  }

  let content;
  if (props.stage <= Stage.PolicyClickNextTags) {
    content = (
      <>
        <Tabs>
          <Tab active={props.stage <= Stage.ClickJSONTab}>Visual Editor</Tab>
          <Tab active={props.stage >= Stage.ShowJSONEditor}>JSON</Tab>
        </Tabs>

        {jsonEditor}

        <Footer>
          <NextButton>Next: Tags</NextButton>
        </Footer>
      </>
    );
  }

  if (
    props.stage >= Stage.PolicyTags &&
    props.stage <= Stage.PolicyClickNextReview
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

  if (props.stage >= Stage.PolicyReview) {
    content = (
      <>
        <Text fontSize={4} mb={4}>
          Review policy
        </Text>

        <Section>
          <SectionTitle>Name*</SectionTitle>

          <SectionContent>
            <SectionDropdown>
              <SectionDropdownSelected>
                {props.stage >= Stage.PolicyHasName ? (
                  'SomePolicyName'
                ) : (
                  <>&nbsp;</>
                )}
              </SectionDropdownSelected>
            </SectionDropdown>
          </SectionContent>
        </Section>

        <Footer>
          <NextButton>Create policy</NextButton>
        </Footer>
      </>
    );
  }

  return (
    <AWSWrapper>
      <Page>
        <Content>
          <Header>Create policy</Header>

          {content}
        </Content>
      </Page>
    </AWSWrapper>
  );
}
