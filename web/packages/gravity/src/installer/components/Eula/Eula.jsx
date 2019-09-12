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
import styled from 'styled-components';
import PropTypes from 'prop-types';
import { Flex, Text, ButtonPrimary } from 'design';
import CheckBox from 'gravity/components/CheckBox';
import Logo from './../Logo';
import { AppLayout, StepLayout } from '../Layout';

export default function Eula(props) {
  const { onAccept, app, config } = props;
  const { eula, displayName, logo, } = app;
  const { eulaAgreeText, eulaHeaderText, eulaContentLabelText, } = config;
  const headerText = eulaHeaderText.replace('{0}', displayName);
  const [ accepted, setAccepted ] = React.useState(false);

  function onToggleAccepted(value){
    setAccepted(value);
  }

  return (
    <AppLayout flexDirection="column" px="40px" py="40px">
      <Flex alignItems="center" mb="8">
        <Logo src={logo}/>
        <Text typography="h2"> {headerText} </Text>
      </Flex>
      <StepLayout title={eulaContentLabelText} overflow="auto">
        <StyledAgreement flex="1" px="2" py="2"  mb="4" as={Flex}
          typography="body2"
          mono
          bg="light"
          color="text.onLight">
          {eula}
        </StyledAgreement>
        <CheckBox
          mb="10"
          value={accepted}
          onChange={onToggleAccepted}
          label={eulaAgreeText}
        />
        <ButtonPrimary width="200px" onClick={onAccept} disabled={!accepted} >
          Accept AGREEMENT
        </ButtonPrimary>
      </StepLayout>
    </AppLayout>
  );
}

Eula.propTypes = {
  onAccept: PropTypes.func.isRequired,
  app:  PropTypes.object.isRequired,
  config:  PropTypes.object.isRequired,
}

const StyledAgreement = styled(Text)`
  border-radius: 6px;
  min-height: 200px;
  overflow: auto;
  white-space: pre;
  word-break: break-all;
  word-wrap: break-word;
`;