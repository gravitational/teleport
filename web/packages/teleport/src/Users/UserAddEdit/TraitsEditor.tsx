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

import React, { useEffect, Dispatch, SetStateAction } from 'react';
import { ButtonBorder, Box, Flex, Text, ButtonIcon } from 'design';
import { Add, Trash } from 'design/Icon';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField, requiredAll } from 'shared/components/Validation/rules';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { AllUserTraits } from 'teleport/services/user';

const availableTraitNames = [
  'aws_role_arns',
  'azure_identities',
  'db_names',
  'db_roles',
  'db_users',
  'gcp_service_accounts',
  'host_user_gid',
  'host_user_uid',
  'kubernetes_groups',
  'kubernetes_users',
  'logins',
  'windows_logins',
];

export function TraitsEditor({
  allTraits,
  attempt,
  configuredTraits,
  setConfiguredTraits,
}: TraitEditorProps) {
  useEffect(() => {
    const newTraits = traitsToTraitsOption(allTraits);
    setConfiguredTraits(newTraits);
  }, [allTraits]);

  type InputOption = {
    labelField: 'traitKey' | 'traitValues';
    option: Option | Option[];
    index: number;
  };

  function handleInputChange(i: InputOption) {
    let newTraits = [...configuredTraits];
    if (i.labelField === 'traitValues') {
      let traitValue: Option[] = i.option as Option[];
      newTraits[i.index] = {
        ...newTraits[i.index],
        [i.labelField]: [...traitValue],
      };
      setConfiguredTraits(newTraits);
    } else {
      let traitName: Option = i.option as Option;
      newTraits[i.index] = {
        ...newTraits[i.index],
        [i.labelField]: traitName,
      };
      setConfiguredTraits(newTraits);
    }
  }

  function addTrait() {
    let newTraits = [...configuredTraits];
    newTraits.push(emptyTrait);
    setConfiguredTraits(newTraits);
  }

  function removeTrait(index: number) {
    let newTraits = [...configuredTraits];
    newTraits.splice(index, 1);
    setConfiguredTraits(newTraits);
  }

  const addLabelText =
    configuredTraits.length > 0 ? 'Add another user trait' : 'Add user trait';

  const requireNoDuplicateTraits = (enteredTrait: Option) => () => {
    let k = configuredTraits.map(trait => trait.traitKey.value.toLowerCase());
    let occurance = 0;
    k.forEach(key => {
      if (key === enteredTrait.value.toLowerCase()) {
        occurance++;
      }
    });
    if (occurance > 1) {
      return { valid: false, message: 'Trait key should be unique for a user' };
    }
    return { valid: true };
  };

  return (
    <Box>
      <Text fontSize={1}>User Traits</Text>
      <Box>
        {configuredTraits.map(({ traitKey, traitValues }, index) => {
          return (
            <Box mb={-5} key={index}>
              <Flex alignItems="start" mt={-3} justify="start">
                <Box width="290px" mr={1} mt={4}>
                  <FieldSelectCreatable
                    data-testid="trait-key"
                    options={availableTraitNames.map(r => ({
                      value: r,
                      label: r,
                    }))}
                    placeholder="Select or type new trait name and enter"
                    autoFocus
                    isSearchable
                    value={traitKey}
                    label="Key"
                    rule={requiredAll(
                      requiredField('Trait key is required'),
                      requireNoDuplicateTraits
                    )}
                    onChange={e => {
                      handleInputChange({
                        option: e,
                        labelField: 'traitKey',
                        index: index,
                      });
                    }}
                    createOptionPosition="last"
                    isDisabled={attempt.status === 'processing'}
                  />
                </Box>
                <Box width="400px" ml={3}>
                  <FieldSelectCreatable
                    data-testid="trait-value"
                    mt={4}
                    ariaLabel="trait-values"
                    css={`
                      background: ${props => props.theme.colors.levels.surface};
                    `}
                    placeholder="Type a new trait value and enter"
                    defaultValue={traitValues.map(r => ({
                      value: r,
                      label: r,
                    }))}
                    label="Value"
                    isMulti
                    isSearchable
                    isClearable={false}
                    value={traitValues}
                    rule={requiredField('Trait value cannot be empty')}
                    onChange={e => {
                      handleInputChange({
                        option: e,
                        labelField: 'traitValues',
                        index: index,
                      });
                    }}
                    formatCreateLabel={(i: string) =>
                      'Trait value: ' + `"${i}"`
                    }
                    isDisabled={attempt.status === 'processing'}
                  />
                </Box>
                <ButtonIcon
                  ml={1}
                  mt={7}
                  size={1}
                  title="Remove Trait"
                  aria-label="Remove Trait"
                  onClick={() => removeTrait(index)}
                  css={`
                    &:disabled {
                      opacity: 0.65;
                      pointer-events: none;
                    }
                  `}
                  disabled={attempt.status === 'processing'}
                >
                  <Trash size="medium" />
                </ButtonIcon>
              </Flex>
            </Box>
          );
        })}
      </Box>

      <Box mt={5}>
        <ButtonBorder
          onClick={addTrait}
          label={addLabelText}
          css={`
            padding-left: 12px;
            &:disabled {
              .icon-add {
                opacity: 0.35;
              }
              pointer-events: none;
            }
          `}
          disabled={attempt.status === 'processing'}
        >
          <Add
            className="icon-add"
            size={12}
            css={`
              margin-right: 3px;
            `}
          />
          {addLabelText}
        </ButtonBorder>
      </Box>
    </Box>
  );
}

export function traitsToTraitsOption(allTraits: AllUserTraits): TraitsOption[] {
  let newTrait = [];
  for (let trait in allTraits) {
    if (!allTraits[trait][0]) {
      continue;
    }
    if (allTraits[trait].length > 0) {
      newTrait.push({
        trait: { value: trait, label: trait },
        traitValues: allTraits[trait].map(t => ({
          value: t,
          label: t,
        })),
      });
    }
  }
  return newTrait;
}

export const emptyTrait = {
  traitKey: { value: '', label: 'Select or type new trait name and enter' },
  traitValues: [],
};

export type TraitsOption = { traitKey: Option; traitValues: Option[] };

export type TraitEditorProps = {
  allTraits: AllUserTraits;
  setConfiguredTraits: Dispatch<SetStateAction<TraitsOption[]>>;
  configuredTraits: TraitsOption[];
  attempt: Attempt;
};
