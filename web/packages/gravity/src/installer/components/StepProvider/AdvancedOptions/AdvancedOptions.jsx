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
import { Flex, Box, Text, ButtonIcon } from 'design';
import { ArrowDown, ArrowUp } from 'design/Icon';
import ClusterTags from './ClusterTags';

function AdvancedOptions({children, onChangeTags, ...styles}) {
  const [ isExpanded, setExpanded ] = React.useState(false);

  function onToggle(){
    setExpanded(!isExpanded);
  }

  const IconCmpt = isExpanded ? ArrowUp : ArrowDown;

  return (
    <Flex width="100%" flexDirection="column" bg="primary.light" {...styles}>
      <StyledHeader height="50px" pl="3" pr="2" py="2" flex="1" bg="primary.main" alignItems="center" justifyContent="space-between"
        onClick={onToggle}
      >
        <Text typography="subtitle1" caps>
          Additional Options
        </Text>
        <ButtonIcon onClick={onToggle}>
          <IconCmpt />
        </ButtonIcon>
      </StyledHeader>
      { isExpanded && (
        <Box p="3">
          {children}
          <ClusterTags onChange={onChangeTags}/>
        </Box>
      )}
    </Flex>
  );
}

const StyledHeader = styled(Flex)`
  cursor: pointer;
  // prevent text selection on accidental double click
  -webkit-user-select: none;
  -moz-user-select: none;
  -khtml-user-select: none;
  -ms-user-select: none;
`
export default AdvancedOptions;
