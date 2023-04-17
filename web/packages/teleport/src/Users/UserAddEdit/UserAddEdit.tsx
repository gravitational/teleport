/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { ButtonPrimary, ButtonSecondary, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import Validation from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

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
    roles,
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

  const selectOptions: Option[] = roles.map(r => ({
    value: r,
    label: r,
  }));

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
            maxWidth: '500px',
            width: '100%',
            overflow: 'initial',
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
            <FieldSelect
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
              options={selectOptions}
              elevated={true}
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
