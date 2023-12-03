/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import styled from 'styled-components';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { Text, Toggle } from 'design';

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
            background: inherit;
            font-size: 14px;
          }
        `}
        rule={requiredField('Email is required')}
        value={formValues.email}
        onChange={e => updateFormField('email', e.target.value)}
      />
      <FieldInput
        label="Company Name (optional)"
        readonly={disabled}
        css={`
          input {
            background: inherit;
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
        `}
        rule={requiredField('Suggestions are required')}
        readOnly={disabled}
        value={formValues.feedback}
        onChange={e => updateFormField('feedback', e.target.value)}
        placeholder="Type your suggestions here"
      />
      <ToggleWithCustomStyling
        disabled={disabled}
        isToggled={formValues.newsletterEnabled}
        onToggle={() => {
          updateFormField('newsletterEnabled', !formValues.newsletterEnabled);
        }}
      >
        <Text ml={2} color="text.main">
          Sign me up for the newsletter
        </Text>
      </ToggleWithCustomStyling>
      <ToggleWithCustomStyling
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
          color="text.main"
          css={`
            line-height: 18px;
          `}
        >
          I would like a demo of Teleport&nbsp;Enterprise features
        </Text>
      </ToggleWithCustomStyling>
    </>
  );
}

// Custom styling for the toggle to make it readable on a light background.
// TODO(gzdunek): remove when design team finish work on this form control.
const ToggleWithCustomStyling = styled(Toggle)`
  > div:first-of-type {
    border: 1px solid ${props => props.theme.colors.spotBackground[1]};
  }
`;
