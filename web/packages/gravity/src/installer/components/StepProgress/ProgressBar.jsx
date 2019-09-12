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
import { Flex } from 'design';

export default function ProgressBar(props){
  const { value=0, ...styles } = props;
  return (
    <StyledProgressBar height="18px" {...styles}
      value={value}
      isCompleted={false}
    >
      <span/>
    </StyledProgressBar>
  );
}

ProgressBar.propTypes = {
  value: PropTypes.number.isRequired
}

const StyledProgressBar = styled(Flex)`
  align-items: center;
  flex-shrink: 0;

  background-color: ${ ({theme}) => theme.colors.light};
  border-radius: 12px;
  > span {
    border-radius: 12px;
    ${({theme, value}) => `
      height: 100%;
      width: ${value}%;
      background-color: ${theme.colors.success};
    `}
  }
`;