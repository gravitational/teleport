import React, { useState, useEffect } from 'react';
import { ButtonPrimary, ButtonSecondary, Alert, Box } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import Validation, { Validator } from 'shared/components/Validation';
import { Option } from 'shared/components/Select';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import {
  AccessListMember,
  AccessListRequires,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { EligibleUsersFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';

import { CalendarDateSelect } from '../../Shared';
import {
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
} from '../Shared';

type Props = {
  onClose(): void;
  membershipRequires: AccessListRequires;
  existingMembers: AccessListMember[];
  userOptions: UserOption[];
  fetchAccessList(): Promise<void | boolean>;
};

type UserOption = Option<User>;

export function EnrollNewMembers({
  onClose,
  membershipRequires,
  userOptions,
  existingMembers,
  fetchAccessList,
}: Props) {
  const ctx = useTeleport();

  const { attempt, setAttempt } = useAttempt('');

  const [eligibleUsers, setEligibleUsers] = useState<Option[]>([]);
  const [selectedMembers, setSelectedMembers] = useState<Option[]>([]);
  const [reason, setReason] = useState('');
  const [expiry, setExpiry] = useState<Date>();

  // duplicatedMembers are duplicate members extracted from
  // selectedMembers.
  const [duplicatedMembers, setDuplicatedMembers] = useState<string[]>([]);

  useEffect(() => {
    const filteredMembers = getEligibleUsersForAddingNewUsers(
      membershipRequires,
      userOptions,
      existingMembers
    );

    setEligibleUsers(filteredMembers);
  }, []);

  function handleOnCreate(validator: Validator) {
    setDuplicatedMembers([]);

    if (!validator.validate()) {
      return;
    }

    const { duplicateUsers, newUsers } =
      getNewAndExistingUsersForAddingNewUsers(existingMembers, selectedMembers);

    if (duplicateUsers.length > 0) {
      setDuplicatedMembers(duplicateUsers);
      return;
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList({
        members: [
          ...existingMembers,
          ...newUsers.map(m => ({
            name: m.value,
            joined: new Date(),
            reason,
            addedBy: ctx.storeUser.getUsername(),
            expiry,
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
            <DialogTitle>Enroll New Members</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            {duplicatedMembers.length > 0 && (
              <Alert
                kind="danger"
                children={`The following usernames are already \
                enrolled. Remove them from the list to continue: \
                ${duplicatedMembers.join(', ')}`}
              />
            )}
            <EligibleUsersFieldSelectAndCreate
              autoFocus={true}
              selected={selectedMembers || []}
              isDisabled={attempt.status === 'processing'}
              onChange={vals => setSelectedMembers(vals || [])}
              options={eligibleUsers}
              label="Add Eligible Members"
              requiredErrMsg="Eligible members are required"
              noEligibleUsersFromNoAccess={userOptions.length === 0}
            />
            <Box mb={4}>
              <CalendarDateSelect
                date={expiry}
                onChange={(newDate: Date) => setExpiry(newDate)}
                label="Member Expires (Optional)"
              />
            </Box>
            <FieldTextArea
              label="Reason for Enrolling (Optional)"
              value={reason}
              onChange={e => setReason(e.target.value)}
              fontSize={2}
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
