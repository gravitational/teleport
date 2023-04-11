import React from 'react';
import styled from 'styled-components';

import { Stage } from '../stages';

import { NextButton } from './common';

import type { CommonIAMProps } from './common';

const Form = styled.div`
  margin-top: 20px;
  display: flex;
  flex-direction: column;
`;

const InputContainer = styled.div`
  display: flex;
  margin-bottom: 20px;
`;

const Button = styled.div`
  background: linear-gradient(#fff, #dedede);
  border: 1px solid #b8b8b8;
  color: #444;
  padding: 3px 7px;
  border-radius: 5px;
  font-weight: 700;
  height: 32px;
  box-sizing: border-box;
`;

const Input = styled.div`
  font-size: 14px;
  border-radius: 5px;
  border: 1px solid #ccc;
  padding: 3px 7px;
  height: 32px;
  box-sizing: border-box;
  width: 300px;
  margin-right: 15px;
  background: ${p => (p.disabled ? '#eeeeee' : 'white')};
`;

const ThumbprintContainer = styled.div`
  background: #eeeeee;
  border: 1px solid #cccccc;
  padding: 7px 10px;
  display: flex;
  width: 400px;
  margin-bottom: 20px;
`;

const ThumbprintSection = styled.div`
  margin-right: 60px;
`;

const Thumbprint = styled.span`
  background: ${p => (p.selected ? '#add0f7' : 'none')};
`;

const Buttons = styled.div`
  display: flex;
  justify-content: flex-end;
  width: calc(100% - 120px);
  margin-top: 20px;
`;

export function OpenIDForm(props: CommonIAMProps) {
  const providerURL =
    props.stage >= Stage.PastedProviderURL
      ? 'https://teleport.lol'
      : 'https://';
  const audience =
    props.stage >= Stage.PastedAudience ? 'discover.teleport' : '';

  let providerDisabled = false;
  let buttonText = 'Get thumbprint';
  if (props.stage >= Stage.ThumbprintLoading) {
    buttonText = 'Loading...';
  }
  if (props.stage >= Stage.ThumbprintResult) {
    buttonText = 'Edit URL';
    providerDisabled = true;
  }

  let fingerprintResult;
  if (props.stage >= Stage.ThumbprintResult) {
    fingerprintResult = (
      <div>
        <strong>Verify thumbprint</strong>

        <ThumbprintContainer>
          <ThumbprintSection>
            <div>Thumbprint</div>

            <Thumbprint selected={props.stage === Stage.ThumbprintSelected}>
              examplevaluehere
            </Thumbprint>
          </ThumbprintSection>
          <ThumbprintSection>
            <div>Issued By</div>
            Example Inc
          </ThumbprintSection>
        </ThumbprintContainer>
      </div>
    );
  }

  return (
    <Form>
      <div>Provider URL</div>

      <InputContainer>
        <Input disabled={providerDisabled}>{providerURL}</Input>

        <Button>{buttonText}</Button>
      </InputContainer>

      {fingerprintResult}

      <div>Audience</div>

      <InputContainer>
        <Input>{audience}</Input>
      </InputContainer>

      <Buttons>
        <NextButton>Add provider</NextButton>
      </Buttons>
    </Form>
  );
}
