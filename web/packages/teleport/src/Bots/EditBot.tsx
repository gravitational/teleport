/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { Alert, ButtonSecondary, ButtonWarning } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';

import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import Validation from 'shared/components/Validation';
import { Option } from 'shared/components/Select';

import { EditBotProps } from 'teleport/Bots/types';

export function EditBot({
  fetchRoles,
  attempt,
  name,
  onClose,
  onEdit,
  selectedRoles,
  setSelectedRoles,
}: EditBotProps) {
  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Edit Bot</DialogTitle>
      </DialogHeader>
      <Validation>
        <DialogContent width="450px">
          {attempt.status === 'failed' && (
            <Alert children={attempt.statusText} />
          )}
          <FieldInput
            label="Name"
            placeholder="Name"
            autoFocus
            value={name}
            readonly={true}
            onChange={() => {}}
          />
          <FieldSelectAsync
            menuPosition="fixed"
            label="Bot Roles"
            rule={requiredField('At least one role is required')}
            placeholder="Click to select roles"
            isSearchable
            isMulti
            isSimpleValue
            isClearable={false}
            value={selectedRoles.map(r => ({
              value: r,
              label: r,
            }))}
            onChange={(values: Option[]) =>
              setSelectedRoles(values?.map(v => v.value) || [])
            }
            loadOptions={async input => {
              const roles = await fetchRoles(input);
              return roles.map(r => ({
                value: r,
                label: r,
              }));
            }}
            noOptionsMessage={() => 'No roles found'}
            elevated={true}
          />
        </DialogContent>
      </Validation>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onEdit}
        >
          Save
        </ButtonWarning>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
