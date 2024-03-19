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
import { Box, Flex, ButtonIcon, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { useValidation, Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { ResourceLabel } from 'teleport/services/agents';

export function LabelsInput({
  labels = [],
  setLabels,
  disableBtns = false,
  autoFocus = false,
}: {
  labels: ResourceLabel[];
  setLabels(l: ResourceLabel[]): void;
  disableBtns?: boolean;
  autoFocus?: boolean;
}) {
  const validator = useValidation() as Validator;

  function addLabel() {
    setLabels([...labels, { name: '', value: '' }]);
  }

  function removeLabel(index: number) {
    if (labels.length === 1) {
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

  return (
    <>
      {labels.length > 0 && (
        <Flex mt={2}>
          <Box width="350px" mr="3">
            Label for Resources the User Can Access{' '}
            <span css={{ fontSize: '12px', fontWeight: 'lighter' }}>
              (required field)
            </span>
          </Box>
          <Box>
            Label Value{' '}
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
              <Flex alignItems="center">
                <FieldInput
                  Input
                  rule={requiredUniqueKey}
                  autoFocus={autoFocus}
                  value={label.name}
                  placeholder="label key"
                  width="350px"
                  mr={3}
                  mb={0}
                  onChange={e => handleChange(e, index, 'name')}
                  readonly={disableBtns}
                />
                <FieldInput
                  rule={requiredField('required')}
                  value={label.value}
                  placeholder="label value"
                  width="350px"
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                  readonly={disableBtns}
                />
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
            </Box>
          );
        })}
      </Box>
      <ButtonSecondary
        onClick={e => {
          e.preventDefault();
          addLabel();
        }}
        css={`
          text-transform: none;
          font-weight: normal;
          &:disabled {
            .icon-add {
              opacity: 0.35;
            }
            pointer-events: none;
          }
          &:hover {
          }
        `}
        disabled={disableBtns}
      >
        <Icons.Add
          className="icon-add"
          disabled={disableBtns}
          size="small"
          css={`
            margin-top: -2px;
            margin-right: 3px;
          `}
        />
        Add Another Label
      </ButtonSecondary>
    </>
  );
}
