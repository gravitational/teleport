import React from 'react';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { Text } from 'design';
import Toggle from 'teleport/components/Toggle';
import { ShareFeedbackFormValues } from './types';

interface ShareFeedbackFormProps {
  formValues: ShareFeedbackFormValues;

  setFormValues(values: ShareFeedbackFormValues): void;
}

export function ShareFeedbackForm({
  formValues,
  setFormValues,
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
        css={`
          input {
            font-size: 14px;
          }
        `}
        rule={requiredField('Email is required')}
        value={formValues.email}
        onChange={e => updateFormField('email', e.target.value)}
      />
      <FieldTextArea
        mt={1}
        label="Any suggestions?"
        textAreaCss={`
                font-size: 14px;
              `}
        value={formValues.feedback}
        onChange={e => updateFormField('feedback', e.target.value)}
        placeholder="Type your suggestions here"
      />
      <Toggle
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
          I want your sales team to demo me Teleport&nbsp;Enterprise features
        </Text>
      </Toggle>
    </>
  );
}
