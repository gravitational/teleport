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

import { Dispatch, SetStateAction } from 'react';

import { Box, ButtonBorder, ButtonIcon, Flex, Text } from 'design';
import { Add, Trash } from 'design/Icon';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredAll, requiredField } from 'shared/components/Validation/rules';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { AllUserTraits } from 'teleport/services/user';

/**
 * traitsPreset is a list of system defined traits in Teleport.
 * The list is used to populate traits key option.
 */
const traitsPreset = [
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
  'github_orgs',
];

/**
 * TraitsEditor supports add, edit or remove traits functionality.
 * @param attempt attempt is Attempt status.
 * @param configuredTraits holds traits configured for user in current editor.
 * @param setConfiguredTraits sets user traits in current editor.
 */
export function TraitsEditor({
  attempt,
  configuredTraits,
  setConfiguredTraits,
}: TraitEditorProps) {
  function handleInputChange(i: InputOption | InputOptionArray) {
    const newTraits = [...configuredTraits];
    if (i.labelField === 'traitValues') {
      const traitValue: Option[] = i.option;
      if (traitValue) {
        if (traitValue[traitValue.length - 1].value.trim() === '') {
          return;
        }
        traitValue[traitValue.length - 1].label =
          traitValue[traitValue.length - 1].label.trim();
        traitValue[traitValue.length - 1].value =
          traitValue[traitValue.length - 1].value.trim();
      }
      newTraits[i.index] = {
        ...newTraits[i.index],
        [i.labelField]: traitValue ?? [],
      };
      setConfiguredTraits(newTraits);
    } else {
      const traitName: Option = i.option;
      traitName.label = traitName.label.trim();
      traitName.value = traitName.value.trim();
      newTraits[i.index] = {
        ...newTraits[i.index],
        [i.labelField]: traitName,
      };
      setConfiguredTraits(newTraits);
    }
  }

  function addNewTraitPair() {
    setConfiguredTraits([...configuredTraits, emptyTrait]);
  }

  function removeTraitPair(index: number) {
    const newTraits = [...configuredTraits];
    newTraits.splice(index, 1);
    setConfiguredTraits(newTraits);
  }

  const addLabelText =
    configuredTraits.length > 0 ? 'Add another user trait' : 'Add user trait';

  return (
    <Box>
      <Text typography="body3">User Traits</Text>
      <Box>
        {configuredTraits.map(({ traitKey, traitValues }, index) => {
          return (
            <Box mb={-5} key={index}>
              <Flex alignItems="start" mt={-3}>
                <Box width="290px" mr={1} mt={4}>
                  <FieldSelectCreatable
                    data-testid="trait-key"
                    options={traitsPreset.map(r => ({
                      value: r,
                      label: r,
                    }))}
                    placeholder="Type a trait name and press enter"
                    autoFocus
                    isSearchable
                    value={traitKey}
                    label="Key"
                    rule={requiredAll(
                      requiredField('Trait key is required'),
                      requireNoDuplicateTraits(configuredTraits)
                    )}
                    onChange={e => {
                      handleInputChange({
                        option: e as Option,
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
                    placeholder="Type a trait value and press enter"
                    label="Value"
                    isMulti
                    isSearchable
                    isClearable={false}
                    value={traitValues}
                    rule={requiredField('Trait value cannot be empty')}
                    onChange={e => {
                      handleInputChange({
                        option: e as Option[],
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
                  onClick={() => removeTraitPair(index)}
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
          onClick={addNewTraitPair}
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

type InputOption = {
  labelField: 'traitKey';
  option: Option;
  index: number;
};

type InputOptionArray = {
  labelField: 'traitValues';
  option: Option[];
  index: number;
};

const requireNoDuplicateTraits =
  (configuredTraits: TraitsOption[]) => (enteredTrait: Option) => () => {
    const traitKey = configuredTraits.map(trait =>
      trait.traitKey.value.toLowerCase()
    );
    let occurance = 0;
    traitKey.forEach(key => {
      if (key === enteredTrait.value.toLowerCase()) {
        occurance++;
      }
    });
    if (occurance > 1) {
      return { valid: false, message: 'Trait key should be unique for a user' };
    }
    return { valid: true };
  };

export const emptyTrait = {
  traitKey: { value: '', label: 'Type a trait name and press enter' },
  traitValues: [],
};

export type TraitsOption = { traitKey: Option; traitValues: Option[] };

export type TraitEditorProps = {
  setConfiguredTraits: Dispatch<SetStateAction<TraitsOption[]>>;
  configuredTraits: TraitsOption[];
  attempt: Attempt;
};

export function traitsToTraitsOption(allTraits: AllUserTraits): TraitsOption[] {
  const newTrait = [];
  for (let trait in allTraits) {
    if (!allTraits[trait]) {
      continue;
    }
    if (allTraits[trait].length === 1 && !allTraits[trait][0]) {
      continue;
    }
    if (allTraits[trait].length > 0) {
      newTrait.push({
        traitKey: { value: trait, label: trait },
        traitValues: allTraits[trait].map(t => ({
          value: t,
          label: t,
        })),
      });
    }
  }
  return newTrait;
}
