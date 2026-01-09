import { useMemo, useState } from 'react';

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
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import { EligibleUsersFieldSelect } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import { CalendarDateSelect } from 'e-teleport/AccessListManagement/Shared/Audit';
import { UserKind } from 'e-teleport/AccessListManagement/Shared/types';
import { useFetch } from 'e-teleport/AccessListManagement/useFetch';
import {
  EnrollingNestedListsAlert,
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

import type { MemberSelection } from '../../Shared/Shared';

type Props = {
  onClose(): void;
  updateAccessList(accessList: AccessList, members?: AccessListMember[]): void;
  accessList: AccessListModified;
};

export function EnrollNewMembersFields({
  selectedMembers,
  setSelectedMembers,
  attempt,
  optional = false,
  userKind,
}: {
  attempt: Attempt;
  selectedMembers: Option<MemberSelection>[];
  setSelectedMembers: (vals: Option<MemberSelection>[]) => void;
  optional?: boolean;
  userKind: UserKind;
}) {
  const { fetchUsersOptions, fetchAccessListsOptions } = useFetch();

  function updateSelectedMembers(
    vals: Option<MemberSelection>[],
    membershipKind: AccessListMemberKind
  ) {
    // if we "create" a user that doesnt exist, the value comes back as a string rather than the formatted
    // value we need. we can update it here.
    const formattedVals = vals.map(val =>
      typeof val.value === 'string' &&
      membershipKind === AccessListMemberKind.User
        ? { label: val.label, value: { membershipKind, name: val.value } }
        : val
    );
    const otherType =
      membershipKind === AccessListMemberKind.User
        ? AccessListMemberKind.List
        : AccessListMemberKind.User;

    // keep existing selections of the opposite type
    const existingOtherTypeSelections = selectedMembers.filter(
      member => member.value.membershipKind === otherType
    );

    // combine new selections with existing selections of the opposite type
    setSelectedMembers([...existingOtherTypeSelections, ...formattedVals]);
  }
  const requiredErrMsg = useMemo(() => {
    if (optional) {
      return '';
    }
    const usersExit = selectedMembers.some(
      m => m.value.membershipKind === AccessListMemberKind.User
    );
    const listsExist = selectedMembers.some(
      m => m.value.membershipKind === AccessListMemberKind.List
    );

    if (!usersExit && !listsExist) {
      return 'Please select at least one user or access list.';
    }

    return '';
  }, [selectedMembers, optional]);

  return (
    <>
      <EligibleUsersFieldSelect
        selected={
          selectedMembers.filter(
            member => member.value.membershipKind === AccessListMemberKind.User
          ) || []
        }
        isDisabled={attempt.status === 'processing'}
        onChange={vals =>
          updateSelectedMembers(vals || [], AccessListMemberKind.User)
        }
        loadOptions={fetchUsersOptions}
        placeholder="Search for a user…"
        label={`Add ${userKind}`}
        requiredErrMsg={requiredErrMsg}
      />
      <EligibleUsersFieldSelect
        userKind="nested-access-list"
        disableCreate
        selected={
          selectedMembers.filter(
            member => member.value.membershipKind === AccessListMemberKind.List
          ) || []
        }
        isDisabled={attempt.status === 'processing'}
        onChange={vals =>
          updateSelectedMembers(vals || [], AccessListMemberKind.List)
        }
        loadOptions={fetchAccessListsOptions}
        placeholder="Search for an access list…"
        noOptionsMsg="No access lists found."
        label={`Add Access Lists as ${userKind}`}
        requiredErrMsg={requiredErrMsg}
      />
    </>
  );
}

export function EnrollNewMembers({
  onClose,
  accessList,
  updateAccessList,
}: Props) {
  const { members: existingMembers } = accessList;
  const ctx = useTeleport();

  const { attempt, setAttempt } = useAttempt('');

  const [selectedMembers, setSelectedMembers] = useState<
    Option<MemberSelection>[]
  >([]);
  const [reason, setReason] = useState('');
  const [expires, setExpires] = useState<Date>();

  // duplicatedMembers are duplicate members extracted from
  // selectedMembers.
  const [duplicatedMembers, setDuplicatedMembers] = useState<string[]>([]);

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
              <Alert kind="danger">{attempt.statusText}</Alert>
            )}
            {duplicatedMembers.length > 0 && (
              <Alert kind="danger">
                {`The following usernames are already enrolled. Remove them from the list to continue: ${duplicatedMembers.join(', ')}`}
              </Alert>
            )}
            <EnrollNewMembersFields
              userKind="Members"
              attempt={attempt}
              selectedMembers={selectedMembers}
              setSelectedMembers={setSelectedMembers}
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
