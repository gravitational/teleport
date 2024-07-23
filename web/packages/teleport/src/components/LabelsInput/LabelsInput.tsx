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
import { Validator, useValidation } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import * as Icons from 'design/Icon';

export type Label = {
  name: string;
  value: string;
};

export type LabelInputTexts = {
  fieldName: string;
  placeholder: string;
};

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
}) {
  const validator = useValidation() as Validator;

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

  const requiredUniqueKey = value => () => {
    // Check for empty length and duplicate key.
    let notValid = !value || value.length === 0;

    return {
      valid: !notValid,
      message: '', // err msg doesn't matter as it isn't diaplsyed.
    };
  };

  const width = `${inputWidth}px`;
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
          return (
            <Box mb={2} key={index}>
              <Flex alignItems="center">
                <FieldInput
                  Input
                  rule={requiredUniqueKey}
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
                  rule={requiredField('required')}
                  value={label.value}
                  placeholder={labelVal.placeholder}
                  width={width}
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                  readonly={disableBtns}
                />
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

const SmallText = styled.span`
  font-size: ${p => p.theme.fontSizes[1]}px;
  font-weight: lighter;
`;
