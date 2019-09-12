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
import  * as Icons from 'design/Icon/Icon';
import { Flex, Text } from 'design';

export default function ProgressDescription(props){
  const { step=0, steps=[], ...styles } = props;

  const items = steps.map((name, index) => ({
    isCompleted: step > index,
    isProcessing: step === index,
    name
  }))


  const groupItems1 = items.slice(0, 3);
  const groupItems2 = items.slice(3, 6);
  const groupItems3 = items.slice(6, 10);

  return (
    <Flex bg="primary.light" justifyCofntent="space-between"  {...styles} >
      <Group IconComponent={Icons.SettingsInputComposite} title="Gathering Instances"
        items={groupItems1} />
      <Group
        IconComponent={Icons.Equalizer}
        title="Configure and install"
        items={groupItems2} />
      <Group
        IconComponent={Icons.ListAddCheck}
        title="Finalizing install"
        items={groupItems3} />
    </Flex>
  );
}

ProgressDescription.propTypes = {
  steps: PropTypes.array.isRequired,
  step: PropTypes.number.isRequired
}

function Group({ title, items, IconComponent}){
  const $items = items.map(item => (
    <Item key={item.name} {...item} />
  ))
  return (
    <StyledGroup flexDirection="column" p="4" flex="1">
      <Flex as={Text} mb="4" typography="h3" alignItems="center">
        <IconComponent mr="3" fontSize="24px" width="50px" style={{ textAlign: "center" }}/>
        {title}
      </Flex>
      <Flex flexDirection="column">
        {$items}
      </Flex>
    </StyledGroup>
  )
}

function Item({ isCompleted, isProcessing, name}){
  let IconCmpt = () => null;
  if(isCompleted){
    IconCmpt = Icons.CircleCheck;
  }

  if(isProcessing){
    IconCmpt =  StyledSpinner;
  }

  return (
    <Flex as={Text} typography="h5" my="3" alignItems="center" style={{position: "relative"}}>
      <div style={{position: "absolute"}}>
        <IconCmpt ml="3" fontSize="20px"  />
      </div>
      <Text ml="9" >{name}</Text>
    </Flex>
  )
}

const StyledGroup = styled(Flex)`
  border-right: 1px solid ${ ({ theme }) => theme.colors.primary.dark};

  &:last-child{
    border-right: none;
  }
`

const StyledSpinner = styled(Icons.Spinner)`
  ${({fontSize="32px"}) => `
    font-size: ${fontSize};
    height: ${fontSize};
    width: ${fontSize};
  `}

  animation: anim-rotate 2s infinite linear;
  color: #fff;
  display: inline-block;
  opacity: .87;
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