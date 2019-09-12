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
import htmlUtils from 'gravity/lib/htmlUtils';
import { Flex, Text, ButtonPrimary } from 'design';

export default function CmdText({cmd, ...styles}){
  const ref = React.useRef();
  function onCopy(){
    event.preventDefault();
    htmlUtils.copyToClipboard(cmd);
    htmlUtils.selectElementContent(ref.current);
  }

  return (
    <Flex bg="bgTerminal" alignItems="start" style={{borderRadius: "4px"}} {...styles} >
      <StyledCmd ref={ref} typography="body2" px="2" py="3" color="primary.contrastText">{cmd}</StyledCmd>
      <ButtonPrimary m="2" size="small" onClick={onCopy}>copy</ButtonPrimary>
    </Flex>
  )
}

const StyledCmd = styled(Text)`
  font-family: ${ ({theme})=> theme.fonts.mono};
  width: 100%;
  word-break: break-all;
  word-wrap: break-word;
  border-radius: 6px;
`;

CmdText.propTypes = {
  ...Flex.propTypes,
  cmd: PropTypes.string
}