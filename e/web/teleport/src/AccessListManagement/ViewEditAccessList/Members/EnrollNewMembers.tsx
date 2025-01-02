import { useEffect, useMemo, useState } from 'react';

import { Alert, Box, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { EligibleUsersFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import { CalendarDateSelect } from 'e-teleport/AccessListManagement/Shared/Audit';
import {
  convertAccessListsToUserOptions,
  EnrollingNestedListsAlert,
  filterExistingUsersAndConvertToOption,
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
  type AccessListModified,
} from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import {
  AccessList,
  AccessListMember,
  AccessListMemberKind,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'teleport/useTeleport';

import type { MemberSelection, UserOption } from '../../Shared/Shared';

type Props = {
  onClose(): void;
  userOptions: UserOption[];
  updateAccessList(accessList: AccessList, members?: AccessListMember[]): void;
  accessList: AccessListModified;
  accessLists: AccessList[];
};

export function EnrollNewMembers({
  onClose,
  accessList,
  userOptions,
  updateAccessList,
  accessLists,
}: Props) {
  const { id, membershipRequires, members: existingMembers } = accessList;
  const ctx = useTeleport();

  const { attempt, setAttempt } = useAttempt('');

  const [eligibleUsers, setEligibleUsers] = useState<Option<MemberSelection>[]>(
    []
  );
  const [selectedMembers, setSelectedMembers] = useState<
    Option<MemberSelection>[]
  >([]);
  const [reason, setReason] = useState('');
  const [expires, setExpires] = useState<Date>();

  // duplicatedMembers are duplicate members extracted from
  // selectedMembers.
  const [duplicatedMembers, setDuplicatedMembers] = useState<string[]>([]);

  useEffect(() => {
    let filteredMembers: Option<MemberSelection>[] = [];

    // If no required traits or roles are defined,
    // Then all users are allowed to be added, except
    // for users who were already added.
    if (
      membershipRequires.roles.length > 0 ||
      membershipRequires.traitLabels.length > 0
    ) {
      filteredMembers = getEligibleUsersForAddingNewUsers(
        membershipRequires,
        userOptions,
        existingMembers
      );
    } else {
      filteredMembers = filterExistingUsersAndConvertToOption(
        userOptions,
        existingMembers
      );
    }

    filteredMembers = filteredMembers.concat(
      convertAccessListsToUserOptions(id, accessLists, existingMembers)
    );
    setEligibleUsers(filteredMembers);
  }, []);

  const selectedMembersContainAccessLists = useMemo(
    () =>
      selectedMembers.some(
        m => m.value?.membershipKind === AccessListMemberKind.List
      ),
    [selectedMembers]
  );

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

    const currentUsername = ctx.storeUser.getUsername();

    const membersToUse = [
      ...existingMembers,
      ...newUsers.map(newUser => {
        const name =
          typeof newUser.value === 'object' && 'name' in newUser.value
            ? newUser.value.name
            : newUser.value;
        const membershipKind =
          typeof newUser.value === 'object' && 'membershipKind' in newUser.value
            ? newUser.value.membershipKind
            : AccessListMemberKind.User;

        return {
          name,
          joined: new Date(),
          reason,
          addedBy: currentUsername,
          // Lists' expiration date is not checked on the backend
          expires:
            membershipKind !== AccessListMemberKind.List ? expires : undefined,
          membershipKind,
        } satisfies AccessListMember;
      }),
    ];

    accessManagementService
      .updateAccessList({
        original: accessList,
        req: { members: membersToUse },
      })
      .then(resp => {
        onClose();
        updateAccessList(resp, membersToUse);
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
            <DialogTitle>Enroll New Members or Access Lists</DialogTitle>
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
              label="Add Members or Access Lists"
              requiredErrMsg="Members are required"
              noEligibleUsersFromNoAccess={userOptions.length === 0}
            />
            {selectedMembersContainAccessLists && (
              <EnrollingNestedListsAlert
                kind="member"
                listName={accessList.title}
              />
            )}
            <Box mb={4}>
              <CalendarDateSelect
                date={expires}
                onChange={(newDate: Date) => setExpires(newDate)}
                label="Member Expires (Optional)"
              />
            </Box>
            <FieldTextArea
              label="Reason for Enrolling (Optional)"
              value={reason}
              onChange={e => setReason(e.target.value)}
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
