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

import { Box, ButtonIcon, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';
import { ButtonTextWithAddIcon } from 'shared/components/ButtonTextWithAddIcon';
import FieldInput from 'shared/components/FieldInput';
import { useValidation, Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { ResourceLabel } from 'teleport/services/agents';

export function LabelsCreater({
  labels = [],
  setLabels,
  disableBtns = false,
  isLabelOptional = false,
  noDuplicateKey = false,
  autoFocus = false,
}: {
  labels: DiscoverLabel[];
  setLabels(l: DiscoverLabel[]): void;
  disableBtns?: boolean;
  isLabelOptional?: boolean;
  noDuplicateKey?: boolean;
  autoFocus?: boolean;
}) {
  const validator = useValidation() as Validator;

  function addLabel() {
    // Prevent adding more rows if there are
    // empty input fields. After checking,
    // reset the validator so the newly
    // added empty input boxes are not
    // considered an error.
    if (!validator.validate()) {
      return;
    }
    validator.reset();
    setLabels([...labels, { name: '', value: '' }]);
  }

  function removeLabel(index: number) {
    if (!isLabelOptional && labels.length === 1) {
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
    labelField: keyof ResourceLabel
  ) => {
    const { value } = event.target;
    const newList = [...labels];

    // Check for any dup key:
    if (noDuplicateKey && labelField === 'name') {
      const isDupKey = labels.some(l => l.name === value);
      newList[index] = { ...newList[index], [labelField]: value, isDupKey };
    } else {
      newList[index] = { ...newList[index], [labelField]: value };
    }
    setLabels(newList);
  };

  const requiredUniqueKey = value => () => {
    // Check for empty length and duplicate key.
    let notValid = !value || value.length === 0;
    if (noDuplicateKey) {
      notValid = notValid || labels.some(l => l.isDupKey);
    }
    return {
      valid: !notValid,
      message: 'required',
    };
  };

  const inputSize = 'medium';
  return (
    <>
      {labels.length > 0 && (
        <Flex mt={2}>
          <Box width="170px" mr="3">
            Key{' '}
            <span css={{ fontSize: '12px', fontWeight: 'lighter' }}>
              (required field)
            </span>
          </Box>
          <Box>
            Value{' '}
            <span css={{ fontSize: '12px', fontWeight: 'lighter' }}>
              (required field)
            </span>
          </Box>
        </Flex>
      )}
      <Box>
        {labels.map((label, index) => {
          return (
            <Box mb={2} key={index}>
              <Flex alignItems="start">
                <FieldInput
                  size={inputSize}
                  rule={requiredUniqueKey}
                  autoFocus={autoFocus}
                  value={label.name}
                  placeholder="label key"
                  width="170px"
                  mr={3}
                  mb={0}
                  onChange={e => handleChange(e, index, 'name')}
                  readonly={disableBtns || label.isFixed}
                  markAsError={label.isDupKey}
                />
                <FieldInput
                  size={inputSize}
                  rule={requiredField('required')}
                  value={label.value}
                  placeholder="label value"
                  width="170px"
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                  readonly={disableBtns || label.isFixed}
                />
                {!label.isFixed && (
                  // Force the trash button container to be the same height as
                  // an input. We can't just set `alignItems="center"` on the
                  // parent flex container above, because the field can expand
                  // when showing a validation error.
                  <Flex
                    alignItems="center"
                    height={inputGeometry[inputSize].height}
                  >
                    <ButtonIcon
                      size={1}
                      title="Remove Label"
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
                )}
              </Flex>
              {label.isDupKey && (
                <Text color="red" typography="body3">
                  Duplicate key not allowed
                </Text>
              )}
            </Box>
          );
        })}
      </Box>
      <ButtonTextWithAddIcon
        label={labels.length === 0 ? 'Add a Label' : 'Add Another Label'}
        onClick={addLabel}
        disabled={disableBtns}
      />
    </>
  );
}

export type DiscoverLabel = ResourceLabel & {
  // isFixed is a flag to mean label is
  // unmodifiable and undeletable.
  isFixed?: boolean;
  // isDupKey is a flag to mean this label
  // has duplicate key.
  isDupKey?: boolean;
};
