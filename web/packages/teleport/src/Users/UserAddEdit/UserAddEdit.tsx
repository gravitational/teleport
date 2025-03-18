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

import { Alert, Box, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import UserTokenLink from './../UserTokenLink';
import { TraitsEditor } from 'shared/components/TraitsEditor';
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
    setConfiguredTraits,
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
            height: '100%',
            maxHeight: '600px',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <DialogHeader>
            <DialogTitle>{isNew ? 'Create User' : 'Edit User'}</DialogTitle>
          </DialogHeader>
          <DialogContent overflow={'auto'}>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box maxWidth={690}>
              <FieldInput
                mr={2}
                label="Username"
                rule={requiredField('Username is required')}
                placeholder="Username"
                autoFocus
                value={name}
                onChange={e => onChangeName(e.target.value)}
                readonly={isNew ? false : true}
              />
              <FieldSelectAsync
                mr={2}
                menuPosition="fixed"
                label="User Roles"
                rule={requiredField('At least one role is required')}
                placeholder="Click to select roles"
                isSearchable
                isMulti
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
                attempt={attempt}
                configuredTraits={configuredTraits}
                setConfiguredTraits={setConfiguredTraits}
              />
            </Box>
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
