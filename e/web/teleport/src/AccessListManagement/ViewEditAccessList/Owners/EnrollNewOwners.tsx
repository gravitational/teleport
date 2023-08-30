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
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { Option } from 'shared/components/Select';

import {
  AccessListOwner,
  AccessListRequires,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { EligibleUsersFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import { UserOption } from 'e-teleport/AccessListManagement/Shared';

import {
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
} from '../Shared';

type Props = {
  onClose(): void;
  ownershipRequires: AccessListRequires;
  userOptions: UserOption[];
  fetchAccessList(): Promise<void | boolean>;
  existingOwners: AccessListOwner[];
};

export function EnrollNewOwners({
  onClose,
  ownershipRequires,
  userOptions,
  fetchAccessList,
  existingOwners,
}: Props) {
  const { attempt, setAttempt } = useAttempt('');

  const [eligibleUsers, setEligibleUsers] = useState<Option[]>([]);
  const [selectedOwners, setSelectedOwners] = useState<Option[]>([]);
  const [description, setDescription] = useState('');

  // duplicatedOwners are duplicate owners extracted from
  // selectedOwners.
  const [duplicatedOwners, setDuplicatedOwners] = useState<string[]>([]);

  useEffect(() => {
    const filteredOwners = getEligibleUsersForAddingNewUsers(
      ownershipRequires,
      userOptions,
      existingOwners
    );

    setEligibleUsers(filteredOwners);
  }, []);

  function handleOnCreate(validator: Validator) {
    setDuplicatedOwners([]);

    if (!validator.validate()) {
      return;
    }

    const { duplicateUsers, newUsers } =
      getNewAndExistingUsersForAddingNewUsers(existingOwners, selectedOwners);

    if (duplicateUsers.length > 0) {
      setDuplicatedOwners(duplicateUsers);
      return;
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList({
        owners: [
          ...existingOwners,
          ...newUsers.map(o => ({
            name: o.value,
            description,
          })),
        ],
      })
      .then(() => {
        onClose();
        fetchAccessList();
      })
      .catch((e: Error) => {
        setAttempt({ status: 'failed', statusText: e.message });
      });
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
            <DialogTitle>Enroll New Owners</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            {duplicatedOwners.length > 0 && (
              <Alert
                kind="danger"
                children={`The following usernames are already \
                enrolled, remove them from the list to continue: \
                ${duplicatedOwners.join(', ')}`}
              />
            )}
            <EligibleUsersFieldSelectAndCreate
              autoFocus={true}
              selected={selectedOwners || []}
              isDisabled={attempt.status === 'processing'}
              onChange={vals => setSelectedOwners(vals || [])}
              options={eligibleUsers}
              label="Add Eligible List Owners"
              requiredErrMsg="Eligible owners are required"
              noEligibleUsersFromNoAccess={userOptions.length === 0}
            />
            <FieldTextArea
              label="Describe Owner (Optional)"
              value={description}
              onChange={e => setDescription(e.target.value)}
            />
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => handleOnCreate(validator)}
            >
              Enroll New Members
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
