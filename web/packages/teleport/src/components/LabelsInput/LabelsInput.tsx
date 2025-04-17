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

import { ButtonIcon, ButtonSecondary, Flex } from 'design';
import { buttonSizes } from 'design/ButtonIcon';
import * as Icons from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';
import { LabelContent } from 'design/LabelInput/LabelInput';
import FieldInput from 'shared/components/FieldInput';
import {
  useRule,
  useValidation,
  Validator,
} from 'shared/components/Validation';
import {
  precomputed,
  requiredField,
  Rule,
  ValidationResult,
} from 'shared/components/Validation/rules';

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
  results?: LabelValidationResult[];
};

type LabelValidationResult = {
  name: ValidationResult;
  value: ValidationResult;
};

export type LabelsRule = Rule<Label[], LabelListValidationResult>;

const buttonIconSize = 0;

export function LabelsInput({
  legend,
  tooltipContent,
  tooltipSticky,
  labels = [],
  setLabels,
  disableBtns = false,
  autoFocus = false,
  required = false,
  adjective = 'Label',
  labelKey = { fieldName: 'Key', placeholder: 'label key' },
  labelVal = { fieldName: 'Value', placeholder: 'label value' },
  rule = defaultRule,
}: {
  legend?: string;
  tooltipContent?: string;
  tooltipSticky?: boolean;
  labels: Label[];
  setLabels(l: Label[]): void;
  disableBtns?: boolean;
  autoFocus?: boolean;
  adjective?: string;
  labelKey?: LabelInputTexts;
  labelVal?: LabelInputTexts;
  /**
   * Makes it so at least one label is required
   */
  required?: boolean;
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
    if (required && labels.length === 1) {
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

  const inputSize = 'medium';
  return (
    <Fieldset>
      {legend && (
        <Legend>
          <LabelContent
            required={required}
            tooltipContent={tooltipContent}
            tooltipSticky={tooltipSticky}
          >
            {legend}
          </LabelContent>
        </Legend>
      )}
      <LabelTable>
        <colgroup>
          {/* Column elements (for styling purposes, see LabelTable styles) */}
          <col />
          <col />
          <col />
        </colgroup>
        {labels.length > 0 && (
          <thead>
            <tr>
              <th scope="col">
                <LabelContent required>{labelKey.fieldName}</LabelContent>
              </th>
              <th scope="col">
                <LabelContent required>{labelVal.fieldName}</LabelContent>
              </th>
            </tr>
          </thead>
        )}
        <tbody>
          {labels.map((label, index) => {
            const validationItem: LabelValidationResult | undefined =
              validationResult.results?.[index];
            return (
              <tr key={index}>
                <td>
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
                    mb={0}
                    onChange={e => handleChange(e, index, 'name')}
                    readonly={disableBtns}
                  />
                </td>
                <td>
                  <FieldInput
                    size={inputSize}
                    rule={
                      validationItem
                        ? precomputed(validationItem.value)
                        : requiredField('required')
                    }
                    value={label.value}
                    placeholder={labelVal.placeholder}
                    mb={0}
                    onChange={e => handleChange(e, index, 'value')}
                    readonly={disableBtns}
                  />
                </td>
                <td>
                  {/* Force the trash button container to be the same height as an
                      input. We can't just set center-align the cell, because the
                      field can expand when showing a validation error. */}
                  <Flex
                    alignItems="center"
                    height={inputGeometry[inputSize].height}
                  >
                    <ButtonIcon
                      size={buttonIconSize}
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
                      <Icons.Cross color="text.muted" size="small" />
                    </ButtonIcon>
                  </Flex>
                </td>
              </tr>
            );
          })}
        </tbody>
      </LabelTable>
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
    </Fieldset>
  );
}

const defaultRule = () => () => ({ valid: true });

export const nonEmptyLabels: LabelsRule = labels => () => {
  const results = labels.map(label => ({
    name: requiredField('required')(label.name)(),
    value: requiredField('required')(label.value)(),
  }));
  return {
    valid: results.every(r => r.name.valid && r.value.valid),
    results: results,
  };
};

const Fieldset = styled.fieldset`
  border: none;
  margin: 0;
  padding: 0;
`;

const Legend = styled.legend`
  margin: 0 0 ${props => props.theme.space[1]}px 0;
  padding: 0;
  ${props => props.theme.typography.body3}
`;

const LabelTable = styled.table`
  width: 100%;
  border-collapse: collapse;
  /*
   * Using fixed layout seems to be the only way to prevent the internal input
   * padding from somehow influencing the column width. As the padding is
   * variable (and reflects the error state), we'd rather avoid column width
   * changes while editing.
   */
  table-layout: fixed;

  & th {
    padding: 0 0 ${props => props.theme.space[1]}px 0;
  }

  col:nth-child(3) {
    /*
     * The fixed layout is good for stability, but it forces us to explicitly
     * define the width of the delete button column. Set it to the width of an
     * icon button.
     */
    width: ${buttonSizes[buttonIconSize].width};
  }

  & td {
    padding: 0;
    /* Keep the inputs top-aligned to support error messages */
    vertical-align: top;
    padding-bottom: ${props => props.theme.space[2]}px;

    &:nth-child(1),
    &:nth-child(2) {
      padding-right: ${props => props.theme.space[2]}px;
    }
  }
`;
