/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react';
import PropTypes from 'prop-types';
import styled from 'styled-components';
import { StepEnum } from './../store';
import { Flex, Text } from 'design';

export default function Description({store}){
  const { step, config } = store.state;
  const {
    licenseUserHintText,
    prereqUserHintText,
    provisionUserHintText,
    progressUserHintText,
  } = config;


  let hintTexts = {};
  hintTexts[StepEnum.LICENSE] = licenseUserHintText;
  hintTexts[StepEnum.NEW_APP] = prereqUserHintText;
  hintTexts[StepEnum.PROVISION] = provisionUserHintText;
  hintTexts[StepEnum.PROGRESS] = progressUserHintText;

  let text = hintTexts[step] || 'Your custom text here';

  return (
    <StyledHint flexDirection="column" py="10" px="5" maxWidth="600px">
      <Text typography="h3" mb="4">
        About this step
      </Text>
      <Text typography="paragraph"
        dangerouslySetInnerHTML={{ __html: text }}
      />
    </StyledHint>
  )
}

Description.style = {
  whiteSpace: 'pre-line'
}

Description.propTypes = {
  store: PropTypes.object.isRequired
}

const StyledHint = styled(Flex)`
  white-space: pre-line;

  .ul{
    padding-left: 10px
  }
`