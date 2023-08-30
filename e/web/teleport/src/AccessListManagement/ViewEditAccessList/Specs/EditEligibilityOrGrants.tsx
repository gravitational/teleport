import React, { useState, useEffect } from 'react';
import { ButtonPrimary, ButtonSecondary, Alert } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import Validation, { Validator } from 'shared/components/Validation';
import { Option } from 'shared/components/Select';

import {
  AccessList,
  AccessListRequires,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { EditKind } from 'e-teleport/AccessListManagement/Shared';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';

type Props = {
  onClose(): void;
  existingRoles: AccessListRequires['roles'];
  editKind: EditKind;
  roleOptions: Option[];
  fetchAccessList(): Promise<void | boolean>;
};

export function EditEligibilityOrGrantRoles({
  onClose,
  existingRoles,
  editKind,
  roleOptions,
  fetchAccessList,
}: Props) {
  const { attempt, setAttempt } = useAttempt('');
  const [selectedRoles, setSelectedRoles] = useState<Option[]>([]);

  useEffect(() => {
    let selectedRoles = existingRoles.map(r => ({ value: r, label: r }));

    if (roleOptions.length > 0) {
      selectedRoles = roleOptions.filter(roleOpt =>
        existingRoles.includes(roleOpt.value)
      );
    }

    setSelectedRoles(selectedRoles);
  }, []);

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    const roles = selectedRoles.map(r => r.value);
    let req: Partial<AccessList> = {
      ownershipRequires: { roles },
    };
    if (editKind === 'Member') {
      req = {
        membershipRequires: { roles },
      };
    } else if (editKind === 'Grants') {
      req = {
        grants: { roles },
      };
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList(req)
      .then(() => {
        onClose();
        fetchAccessList();
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  let dialogTitle = 'Edit Roles Granted';
  let editBtnTitle = 'Edit Roles Granted';

  if (editKind !== 'Grants') {
    dialogTitle = `Edit ${editKind} Eligibility: Roles Required`;
    editBtnTitle = `Edit ${editKind} Eligibility`;
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({
            maxWidth: '500px',
            width: '100%',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <DialogHeader>
            <DialogTitle>{dialogTitle}</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <EligibilityOrGrantRolesFieldSelectAndCreate
              options={roleOptions}
              isDisabled={attempt.status === 'processing'}
              onChange={(vals: Option[]) => setSelectedRoles(vals || [])}
              selected={selectedRoles}
              autoFocus={true}
              editKind={editKind}
            />
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => handleOnCreate(validator)}
            >
              {editBtnTitle}
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
