import FieldInput from 'shared/components/FieldInput';
import styled from 'styled-components';
import React from 'react';

export const ConfigFieldInput: typeof FieldInput = styled(FieldInput)`
  input {
    background: inherit;
    border: 1px ${props => props.theme.colors.action.disabledBackground} solid;
    color: ${props => props.theme.colors.text.primary};
    box-shadow: none;
    font-size: 14px;
    height: 34px;

    ::placeholder {
      opacity: 1;
      color: ${props => props.theme.colors.text.secondary};
    }

    &:hover {
      border-color: ${props => props.theme.colors.text.secondary};
    }

    &:focus {
      border-color: ${props => props.theme.colors.secondary.main};
    }
  }
`;

const ConfigFieldInputWithoutStepper: typeof ConfigFieldInput = styled(
  ConfigFieldInput
)`
  input {
    ::-webkit-inner-spin-button {
      -webkit-appearance: none;
      margin: 0;
    }

    ::-webkit-outer-spin-button {
      -webkit-appearance: none;
      margin: 0;
    }
  }
`;

export const PortFieldInput: typeof ConfigFieldInput = props => (
  <ConfigFieldInputWithoutStepper
    type="number"
    min={1}
    max={65535}
    {...props}
    width="110px"
  />
);
