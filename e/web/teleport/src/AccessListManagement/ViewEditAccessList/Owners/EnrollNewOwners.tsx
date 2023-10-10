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

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { EligibleUsersFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import { UserOption } from 'e-teleport/AccessListManagement/Shared/Shared';

import {
  filterExistingUsersAndConvertToOption as filterOutExistingUsersAndConvertToOptionType,
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
} from '../Shared';
import { AccessListModified } from '../ViewEditAccessList';

type Props = {
  onClose(): void;
  userOptions: UserOption[];
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
};

export function EnrollNewOwners({
  onClose,
  userOptions,
  fetchAccessList,
  accessList,
}: Props) {
  const { owners: existingOwners, ownershipRequires } = accessList;
  const { attempt, setAttempt } = useAttempt('');

  const [eligibleUsers, setEligibleUsers] = useState<Option[]>([]);
  const [selectedOwners, setSelectedOwners] = useState<Option[]>([]);
  const [description, setDescription] = useState('');

  // duplicatedOwners are duplicate owners extracted from
  // selectedOwners.
  const [duplicatedOwners, setDuplicatedOwners] = useState<string[]>([]);

  useEffect(() => {
    let filteredOwners: Option[] = [];

    // If no required traits or roles are defined,
    // Then all users are allowed to be added, except
    // for users who were already added.
    if (
      ownershipRequires.roles.length > 0 ||
      ownershipRequires.traitLabels.length > 0
    ) {
      filteredOwners = getEligibleUsersForAddingNewUsers(
        ownershipRequires,
        userOptions,
        existingOwners
      );
    } else {
      filteredOwners = filterOutExistingUsersAndConvertToOptionType(
        userOptions,
        existingOwners
      );
    }

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
        req: {
          owners: [
            ...existingOwners,
            ...newUsers.map(o => ({
              name: o.value,
              description,
            })),
          ],
        },
        original: accessList,
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
              label="Add List Owners"
              requiredErrMsg="List Owners are required"
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
              Enroll New Owners
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
