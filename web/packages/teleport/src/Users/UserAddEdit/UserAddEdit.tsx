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

import React, { useState, useEffect } from 'react';
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
    allTraits,
    attempt,
    name,
    selectedRoles,
    onSave,
    isNew,
    token,
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
            <TraitsEditor allTraits={allTraits} />
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
};

function TraitsEditor({ allTraits }: TraitEditorProps) {
  const [tt, setTT] = useState([]);

  const [availableTraits, setAvailableTraits] = useState<Option[]>([])
  useEffect(() => {
    console.log('allTraits: ', allTraits)
    let t = [];
    for (let trait in allTraits) {
      if (allTraits[trait].length === 0) {
        // send empty traits to availableTraits so that users can choose from Trait Name dropdown.
        availableTraits.push({value: trait, label: trait})
        setAvailableTraits(availableTraits)
      }
      if (allTraits[trait].length !== 0) {
        t.push({ trait: trait, traitValues: allTraits[trait] });
      }
    }
    console.log(t);
    setTT(t);
  }, [allTraits]);

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
      {tt.map(({ trait, traitValues }) => {
        return (
          <Box mb={-5} key={trait}>
            <Flex alignItems="center" mt={-3}>
              <Box width="290px" mr={1} mt={4}>
                <FieldSelectCreatable
                options={availableTraits}
                  placeholder="trait-key"
                  autoFocus
                  value={{ value: trait, label: trait }}
                />
              </Box>
              <Box width="400px" ml={3}>
                <FieldSelectCreatable
                  mt={4}
                  ariaLabel="attribute value"
                  css={`
                    background: ${props => props.theme.colors.levels.surface};
                  `}
                  placeholder="trait values"
                  defaultValue={traitValues.map(r => ({
                    value: r,
                    label: r,
                  }))}
                  isMulti
                  value={traitValues.map(r => ({
                    value: r,
                    label: r,
                  }))}
                  isDisabled={false}
                  createOptionPosition="last"
                  formatCreateLabel={(i: string) => `"${i}"`}
                />
              </Box>
              <ButtonIcon
                ml={1}
                size={1}
                title="Remove Attribute"
                onClick={() => null}
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

      <Box mt={4}>
        <ButtonTextWithAddIcon
          onClick={() => null}
          label={'Add another trait'}
          disabled={false}
        />
      </Box>
    </Box>
  );
}
