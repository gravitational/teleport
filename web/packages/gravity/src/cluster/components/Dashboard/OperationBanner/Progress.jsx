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
import { Flex, Text, Box } from 'design';
import { Spinner as SpinnerIcon} from 'design/Icon';
import { Info } from 'design/Alert';
import { NavLink } from 'gravity/components/Router';
import cfg from 'gravity/config';

export default function Progress(props){
  const {  operation } = props;
  const { id, description } = operation;
  const logsUrl = cfg.getSiteLogQueryRoute({query: `file:${id}`});

  return (
    <Info as={Flex} mb="0" justifyContent="space-between" alignItems="center" width="100%">
      <Box mr="2">
        <StyledSpinner fontSize="14px"/>
        <Text ml="2" mr="2" as="span">
          {description} is in progress...
        </Text>
      </Box>
      <Text typography="body1">
        <StyledLink as={NavLink} to={logsUrl}>
          View Details
        </StyledLink>
      </Text>
    </Info>
  )
}

const StyledSpinner = styled(SpinnerIcon)`
  animation: anim-rotate 2s infinite linear;
  font-size: 16px;
  height: 16px;
  opacity: .87;
  width: 16px;
  text-shadow: 0 0 .25em rgba(255,255,255, .3);

  @keyframes anim-rotate {
    0% {
      transform: rotate(0);
    }
    100% {
      transform: rotate(360deg);
    }
  }
`;

const StyledLink = styled.a`
  font-weight: normal;
  background: none;
  text-decoration: underline;
  text-transform: none;
  color: inherit;
`