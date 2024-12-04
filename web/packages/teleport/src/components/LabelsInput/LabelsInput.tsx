/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import styled from 'styled-components';
import { Flex, Box, ButtonSecondary, ButtonIcon } from 'design';
import FieldInput from 'shared/components/FieldInput';
import {
  Validator,
  useRule,
  useValidation,
} from 'shared/components/Validation';
import {
  precomputed,
  requiredField,
  Rule,
  ValidationResult,
} from 'shared/components/Validation/rules';
import * as Icons from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';

export type Label = {
  name: string;
  value: string;
};

export type LabelInputTexts = {
  fieldName: string;
  placeholder: string;
};

type LabelListValidationResult = ValidationResult & {
  /**
   * A list of validation results, one per label. Note: items are optional just
   * because `useRule` by default returns only `ValidationResult`. For the
   * actual validation, it's not optional; if it's undefined, or there are
   * fewer items in this list than the labels, a default validation rule will
   * be used instead.
   */
  items?: LabelValidationResult[];
};

type LabelValidationResult = {
  name: ValidationResult;
  value: ValidationResult;
};

export type LabelsRule = Rule<Label[], LabelListValidationResult>;

export function LabelsInput({
  labels = [],
  setLabels,
  disableBtns = false,
  autoFocus = false,
  areLabelsRequired = false,
  adjective = 'Label',
  labelKey = { fieldName: 'Key', placeholder: 'label key' },
  labelVal = { fieldName: 'Value', placeholder: 'label value' },
  inputWidth = 200,
  rule = defaultRule,
}: {
  labels: Label[];
  setLabels(l: Label[]): void;
  disableBtns?: boolean;
  autoFocus?: boolean;
  adjective?: string;
  labelKey?: LabelInputTexts;
  labelVal?: LabelInputTexts;
  inputWidth?: number;
  /**
   * Makes it so at least one label is required
   */
  areLabelsRequired?: boolean;
  /**
   * A rule for validating the list of labels as a whole. Note that contrary to
   * other input fields, the labels input will default to validating every
   * input as required if this property is undefined.
   */
  rule?: LabelsRule;
}) {
  const validator = useValidation() as Validator;
  const validationResult: LabelListValidationResult = useRule(rule(labels));

  function addLabel() {
    setLabels([...labels, { name: '', value: '' }]);
  }

  function removeLabel(index: number) {
    if (areLabelsRequired && labels.length === 1) {
      // Since at least one label is required
      // instead of removing the last row, clear
      // the input and turn on error.
      const newList = [...labels];
      newList[index] = { name: '', value: '' };
      setLabels(newList);

      validator.validate();
      return;
    }
    const newList = [...labels];
    newList.splice(index, 1);
    setLabels(newList);
  }

  const handleChange = (
    event: React.ChangeEvent<HTMLInputElement>,
    index: number,
    labelField: keyof Label
  ) => {
    const { value } = event.target;
    const newList = [...labels];
    newList[index] = { ...newList[index], [labelField]: value };
    setLabels(newList);
  };

  const requiredKey = value => () => {
    // Check for empty length and duplicate key.
    let notValid = !value || value.length === 0;

    return {
      valid: !notValid,
      message: 'required',
    };
  };

  const width = `${inputWidth}px`;
  const inputSize = 'medium';
  return (
    <>
      {labels.length > 0 && (
        <Flex mt={2}>
          <Box width={width} mr="3">
            {labelKey.fieldName} <SmallText>(required field)</SmallText>
          </Box>
          <Box>
            {labelVal.fieldName} <SmallText>(required field)</SmallText>
          </Box>
        </Flex>
      )}
      <Box>
        {labels.map((label, index) => {
          const validationItem: LabelValidationResult | undefined =
            validationResult.items?.[index];
          return (
            <Box mb={2} key={index}>
              <Flex alignItems="start">
                <FieldInput
                  size={inputSize}
                  rule={
                    validationItem
                      ? precomputed(validationItem.name)
                      : requiredKey
                  }
                  autoFocus={autoFocus}
                  value={label.name}
                  placeholder={labelKey.placeholder}
                  width={width}
                  mr={3}
                  mb={0}
                  onChange={e => handleChange(e, index, 'name')}
                  readonly={disableBtns}
                />
                <FieldInput
                  size={inputSize}
                  rule={
                    validationItem
                      ? precomputed(validationItem.value)
                      : requiredField('required')
                  }
                  value={label.value}
                  placeholder={labelVal.placeholder}
                  width={width}
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                  readonly={disableBtns}
                />
                {/* Force the trash button container to be the same height as an
                    input. We can't just set `alignItems="center"` on the parent
                    flex container above, because the field can expand when
                    showing a validation error. */}
                <Flex
                  alignItems="center"
                  height={inputGeometry[inputSize].height}
                >
                  <ButtonIcon
                    size={1}
                    title={`Remove ${adjective}`}
                    onClick={() => removeLabel(index)}
                    css={`
                      &:disabled {
                        opacity: 0.65;
                        pointer-events: none;
                      }
                    `}
                    disabled={disableBtns}
                  >
                    <Icons.Trash size="medium" />
                  </ButtonIcon>
                </Flex>
              </Flex>
            </Box>
          );
        })}
      </Box>
      <ButtonSecondary
        onClick={e => {
          e.preventDefault();
          addLabel();
        }}
        disabled={disableBtns}
        gap={1}
      >
        <Icons.Add className="icon-add" disabled={disableBtns} size="small" />
        {labels.length > 0 ? `Add another ${adjective}` : `Add a ${adjective}`}
      </ButtonSecondary>
    </>
  );
}

const defaultRule = () => () => ({ valid: true });

const SmallText = styled.span`
  font-size: ${p => p.theme.fontSizes[1]}px;
  font-weight: lighter;
`;

export const nonEmptyLabels: LabelsRule = labels => () => {
  const results = labels.map(label => ({
    name: requiredField('required')(label.name)(),
    value: requiredField('required')(label.value)(),
  }));
  return {
    valid: results.every(r => r.name.valid && r.value.valid),
    items: results,
  };
};
