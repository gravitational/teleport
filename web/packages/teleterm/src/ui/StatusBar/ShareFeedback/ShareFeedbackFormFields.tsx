import React from 'react';
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
