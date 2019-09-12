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
import { Flex, Text } from 'design';

export default function Radio({label, name, checked, onClick, ...styles}){
  return (
    <StyledRadio as="label" {...styles}>
      <input type="radio"
        name={name}
        checked={checked}
        onChange={onClick}
      />
      <Text ml="2" color="primary.contrastText" fontWeight="bold">
        {label}
      </Text>
  </StyledRadio>
  )
}

export function RadioGroup({ selected, name, radioProps, options, onChange, ...props }) {
  const $options = options.map((option, index) => {
    return (
      <Radio
        name={name}
        mr="2"
        mb="2"
        onClick={() => onChange(option)}
        key={index}
        checked={option.value === selected.value}
        label={option.label}
        {...radioProps}
      />
    )
  });

  return (
    <Flex {...props}>
      {$options}
    </Flex>
  );
}

const StyledRadio = styled(Flex)`
  display: inline-flex;
  align-items: center;
  cursor: pointer;
  input {
    -webkit-appearance: none;
    margin: 0;
    background: white;
    border: 2px solid ${ props => props.theme.colors.secondary.main};
    border-radius: 1000px;
    height: 20px;
    width: 20px;
    outline: none;
    padding: 0;
  }

  input:checked {
    position: relative;
    background-color: white;
    box-shadow: 0 2px 16px ${ props => props.theme.colors.secondary.main};
  }

  input:checked::before {
    background-color: ${ props => props.theme.colors.secondary.main };
    position: absolute;
    content: '';
    border-radius: 1000px;
    height: 8px;
    width: 8px;
    top: 4px;
    left: 4px;
  }
`