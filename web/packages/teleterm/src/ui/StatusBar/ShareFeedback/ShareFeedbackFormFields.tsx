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

import { Text, Toggle } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { requiredField } from 'shared/components/Validation/rules';

import { ShareFeedbackFormValues } from './types';

export function ShareFeedbackFormFields({
  formValues,
  setFormValues,
  disabled,
}: {
  disabled: boolean;
  formValues: ShareFeedbackFormValues;
  setFormValues(values: ShareFeedbackFormValues): void;
}) {
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
        rule={requiredField('Suggestions are required')}
        disabled={disabled}
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
        <Text ml={2} color="text.main">
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
          color="text.main"
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
