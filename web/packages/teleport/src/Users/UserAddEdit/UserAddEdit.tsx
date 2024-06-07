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

import React, { useState, useEffect, Dispatch, SetStateAction } from 'react';
import {
  ButtonPrimary,
  ButtonSecondary,
  Alert,
  Box,
  Flex,
  Text,
  ButtonIcon,
} from 'design';
import { ButtonTextWithAddIcon } from 'shared/components/ButtonTextWithAddIcon';
import * as Icons from 'design/Icon';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import Validation from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import {
  FieldSelectAsync,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import { AllUserTraits } from 'teleport/services/user';

import UserTokenLink from './../UserTokenLink';
import useDialog, { Props } from './useDialog';

import type { TraitEditor } from './useDialog';

export default function Container(props: Props) {
  const dialog = useDialog(props);
  return <UserAddEdit {...dialog} />;
}

export function UserAddEdit(props: ReturnType<typeof useDialog>) {
  const {
    onChangeName,
    onChangeRoles,
    onClose,
    fetchRoles,
    setConfiguredTraits,
    allTraits,
    attempt,
    name,
    selectedRoles,
    onSave,
    isNew,
    token,
    configuredTraits,
  } = props;

  if (attempt.status === 'success' && isNew) {
    return <UserTokenLink onClose={onClose} token={token} asInvite={true} />;
  }

  function save(validator) {
    if (!validator.validate()) {
      return;
    }

    onSave();
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({
            maxWidth: '700px',
            width: '100%',
            height: '70%',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <DialogHeader>
            <DialogTitle>{isNew ? 'Create User' : 'Edit User'}</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <FieldInput
              label="Username"
              rule={requiredField('Username is required')}
              placeholder="Username"
              autoFocus
              value={name}
              onChange={e => onChangeName(e.target.value)}
              readonly={isNew ? false : true}
            />
            <FieldSelectAsync
              menuPosition="fixed"
              label="User Roles"
              rule={requiredField('At least one role is required')}
              placeholder="Click to select roles"
              isSearchable
              isMulti
              isSimpleValue
              isClearable={false}
              value={selectedRoles}
              onChange={values => onChangeRoles(values as Option[])}
              noOptionsMessage={() => 'No roles found'}
              loadOptions={async input => {
                const roles = await fetchRoles(input);
                return roles.map(r => ({ value: r, label: r }));
              }}
              elevated={true}
            />
            <TraitsEditor
              allTraits={allTraits}
              configuredTraits={configuredTraits}
              setConfiguredTraits={setConfiguredTraits}
            />
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => save(validator)}
            >
              Save
            </ButtonPrimary>
            <ButtonSecondary
              disabled={attempt.status === 'processing'}
              onClick={onClose}
            >
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}

export type TraitEditorProps = {
  allTraits: AllUserTraits;
  setConfiguredTraits: Dispatch<SetStateAction<TraitEditor[]>>;
  configuredTraits: TraitEditor[];
};

function TraitsEditor({
  allTraits,
  configuredTraits,
  setConfiguredTraits,
}: TraitEditorProps) {

  const [availableTraitNames, setAvailableTraitNames] = useState<Option[]>([]);
  useEffect(() => {
    let newTrait = [];
    for (let trait in allTraits) {
      if (allTraits[trait].length === 0) {
        // send empty traits to availableTraits so that users can choose from Trait Name dropdown.
        availableTraitNames.push({ value: trait, label: trait });
        setAvailableTraitNames(availableTraitNames);
      }
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

    setConfiguredTraits(newTrait);
  }, [allTraits]);

  type InputOption = {
    labelField: 'trait' | 'traitValues';
    option: Option | Option[];
    index: number;
  };

  function handleInputChange(i: InputOption) {
    const newTraits = [...configuredTraits];
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
    const newTraits = [...configuredTraits];
    newTraits.push({trait: {value: '', label: ''}, traitValues: []})
    setConfiguredTraits(newTraits)
  }

  function removeTrait(index: number) {
    const newTraits = [...configuredTraits];
    newTraits.splice(index, 1);
    setConfiguredTraits(newTraits)
  }

  return (
    <Box>
      <Text bold>Traits</Text>
      <Flex mt={2}>
        <Box width="265px">
          <Text fontSize={1}>Trait Name</Text>
        </Box>

        <Text fontSize={1} ml={4}>
          Trait Value
        </Text>
      </Flex>

      <Box>
        {configuredTraits.map(({ trait, traitValues }, index) => {
          return (
            <Box mb={-5} key={index}>
              <Flex alignItems="center" mt={-3}>
                <Box width="290px" mr={1} mt={4}>
                  <FieldSelectCreatable
                    options={availableTraitNames}
                    placeholder="trait-key"
                    autoFocus
                    isSearchable
                    value={trait}
                    onChange={e => {
                      handleInputChange({
                        option: e as Option,
                        labelField: 'trait',
                        index: index,
                      });
                    }}
                  />
                </Box>
                <Box width="400px" ml={3}>
                  <FieldSelectCreatable
                    mt={4}
                    ariaLabel="trait values"
                    css={`
                      background: ${props => props.theme.colors.levels.surface};
                    `}
                    placeholder="trait values"
                    defaultValue={traitValues.map(r => ({
                      value: r,
                      label: r,
                    }))}
                    isMulti
                    isSearchable
                    value={traitValues}
                    onChange={e => {
                      handleInputChange({
                        option: e as Option,
                        labelField: 'traitValues',
                        index: index,
                      });
                    }}
                    isDisabled={false}
                    createOptionPosition="last"
                    formatCreateLabel={(i: string) => `"${i}"`}
                  />
                </Box>
                <ButtonIcon
                  ml={1}
                  size={1}
                  title="Remove Trait"
                  onClick={() => removeTrait(index)}
                  css={`
                    &:disabled {
                      opacity: 0.65;
                      pointer-events: none;
                    }
                  `}
                  disabled={false}
                >
                  <Icons.Trash size="medium" />
                </ButtonIcon>
              </Flex>
            </Box>
          );
        })}
      </Box>

      <Box mt={4}>
        <ButtonTextWithAddIcon
          onClick={addTrait}
          label={'Add another trait'}
          disabled={false}
        />
      </Box>
    </Box>
  );
}
