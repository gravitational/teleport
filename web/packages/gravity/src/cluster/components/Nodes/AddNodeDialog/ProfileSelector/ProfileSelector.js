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
import { Flex, Text, Box, ButtonSecondary, ButtonPrimary } from 'design';
import { DialogContent, DialogFooter} from 'design/DialogConfirmation';

export default function ProfileSelector(props) {
  const { value, options, onChange, onContinue, onClose } = props;
  return (
    <React.Fragment>
      <DialogContent minHeight="200px">
        <Text typography="h6" mb="3" caps color="primary.contrastText">
          Select Profile
        </Text>
        <RadioGroup options={options} selected={value} onChange={onChange}/>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary mr="3" onClick={onContinue}>
          Continue
        </ButtonPrimary>
        <ButtonSecondary onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </React.Fragment>
  );
}

function RadioGroup({ selected, options, onChange, ...props }) {
  const $options = options.map((option, index) => {
    return (
      <Flex key={index} mb="4" as="label" alignItems="center">
        <input type="radio" name="radio"
          onChange={() => onChange(option)}
          checked={option.value === selected.value}
        />
        <Text as="span" ml="3" color="primary.contrastText" fontWeight="bold">
          {option.title}
        </Text>
      </Flex>
    )
  });

  return (
    <StyledRadioGroup {...props}>
      {$options}
    </StyledRadioGroup>
  );
}

const StyledRadioGroup = styled(Box)`

  label {
    cursor: pointer;
  }

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