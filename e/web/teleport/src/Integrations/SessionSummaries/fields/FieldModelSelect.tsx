import { useCallback, useState } from 'react';
import { useFormContext } from 'react-hook-form';
import styled from 'styled-components';

import Box from 'design/Box';

import { FieldInput } from 'e-teleport/Integrations/SessionSummaries/fields/FieldInput';
import {
  FieldRadioGroup,
  type RadioOption,
} from 'e-teleport/Integrations/SessionSummaries/fields/FieldRadioGroup';

interface FieldModelSelectProps {
  options?: RadioOption<string>[];
}

export function FieldModelSelect({ options }: FieldModelSelectProps) {
  const form = useFormContext();

  const [isManualEntry, setIsManualEntry] = useState(
    !options || options?.length === 0
  );

  const handleManualEntryChange = useCallback(() => {
    form.setValue('model', '');

    setIsManualEntry(!isManualEntry);

    if (!isManualEntry) {
      requestAnimationFrame(() => form.setFocus('model'));
    }
  }, [isManualEntry, form]);

  return (
    <Box position="relative">
      {options?.length > 0 && (
        <ToggleManualEntry onClick={handleManualEntryChange}>
          {isManualEntry ? 'Select from list' : 'Enter manually'}
        </ToggleManualEntry>
      )}

      {isManualEntry ? (
        <FieldInput name="model" label="Model" required={true} />
      ) : (
        <FieldRadioGroup
          name="model"
          label="Model"
          options={options}
          required={true}
        />
      )}
    </Box>
  );
}

const ToggleManualEntry = styled.div`
  position: absolute;
  top: 0;
  right: 0;
  color: ${p => p.theme.colors.brand};
  font-size: 12px;
  cursor: pointer;
  user-select: none;

  &:hover {
    text-decoration: underline;
  }
`;
