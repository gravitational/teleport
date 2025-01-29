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

import { Box, ButtonIcon, ButtonSecondary, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { useValidation, Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

export type Label = {
  name: string;
  value: string;
};

export type LabelInputTexts = {
  fieldName: string;
  placeholder: string;
};

export function LabelsInput({
  legend,
  tooltipContent,
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
  legend?: string;
  tooltipContent?: string;
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
    <Fieldset>
      {legend && (
        <Legend>
          {tooltipContent ? (
            <>
              <span
                css={{
                  marginRight: '4px',
                  verticalAlign: 'middle',
                }}
              >
                {legend}
              </span>
              <ToolTipInfo children={tooltipContent} />
            </>
          ) : (
            legend
          )}
        </Legend>
      )}
      {labels.length > 0 && (
        <Flex mt={legend ? 1 : 0} mb={1}>
          <Box width={width} mr="3">
            <Text typography="body2">
              {labelKey.fieldName} (required field)
            </Text>
          </Box>
          <Text typography="body2">{labelVal.fieldName} (required field)</Text>
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
        gap={1}
      >
        <Icons.Add className="icon-add" disabled={disableBtns} size="small" />
        {labels.length > 0 ? `Add another ${adjective}` : `Add a ${adjective}`}
      </ButtonSecondary>
    </Fieldset>
  );
}

const Fieldset = styled.fieldset`
  border: none;
  margin: 0;
  padding: 0;
`;

const Legend = styled.legend`
  margin: 0 0 ${props => props.theme.space[1]}px 0;
  padding: 0;
  ${props => props.theme.typography.body2}
`;
