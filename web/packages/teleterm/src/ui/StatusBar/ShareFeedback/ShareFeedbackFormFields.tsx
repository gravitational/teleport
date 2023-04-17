/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { useTheme } from 'styled-components';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { Text } from 'design';
import Toggle from 'teleport/components/Toggle';

import { ShareFeedbackFormValues } from './types';

interface ShareFeedbackFormProps {
  disabled: boolean;
  formValues: ShareFeedbackFormValues;

  setFormValues(values: ShareFeedbackFormValues): void;
}

export function ShareFeedbackFormFields({
  formValues,
  setFormValues,
  disabled,
}: ShareFeedbackFormProps) {
  function updateFormField<T extends keyof ShareFeedbackFormValues>(
    field: T,
    value: ShareFeedbackFormValues[T]
  ) {
    setFormValues({ ...formValues, [field]: value });
  }

  const theme = useTheme();

  return (
    <>
      <FieldInput
        mt={3}
        type="email"
        label="Email Address"
        autoFocus
        readonly={disabled}
        css={`
          input {
            font-size: 14px;
          }
        `}
        rule={requiredField('Email is required')}
        value={formValues.email}
        onChange={e => updateFormField('email', e.target.value)}
      />
      <FieldInput
        label="Company name (optional)"
        readonly={disabled}
        css={`
          input {
            font-size: 14px;
          }
        `}
        value={formValues.company}
        onChange={e => updateFormField('company', e.target.value)}
      />
      <FieldTextArea
        label="Suggestions"
        textAreaCss={`
        font-size: 14px;
        outline: none;
        color: ${theme.colors.text.primary};
        background: ${theme.colors.levels.surface};
        border: 1px solid ${theme.colors.text.placeholder};
        ::placeholder {
          color: ${theme.colors.text.placeholder};
        }
        &:hover,
        &:focus,
        &:active {
          border: 1px solid ${theme.colors.text.secondary};
        }
        `}
        rule={requiredField('Suggestions are required')}
        readOnly={disabled}
        value={formValues.feedback}
        onChange={e => updateFormField('feedback', e.target.value)}
        placeholder="Type your suggestions here"
      />
      <Toggle
        disabled={disabled}
        isToggled={formValues.newsletterEnabled}
        onToggle={() => {
          updateFormField('newsletterEnabled', !formValues.newsletterEnabled);
        }}
      >
        <Text ml={2} color="text.primary">
          Sign me up for the newsletter
        </Text>
      </Toggle>
      <Toggle
        disabled={disabled}
        isToggled={formValues.salesContactEnabled}
        onToggle={() => {
          updateFormField(
            'salesContactEnabled',
            !formValues.salesContactEnabled
          );
        }}
      >
        <Text
          ml={2}
          color="text.primary"
          css={`
            line-height: 18px;
          `}
        >
          I would like a demo of Teleport&nbsp;Enterprise features
        </Text>
      </Toggle>
    </>
  );
}
