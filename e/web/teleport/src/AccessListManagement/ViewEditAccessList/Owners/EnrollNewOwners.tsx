import { useMemo, useState } from 'react';

import { Alert, ButtonPrimary, ButtonSecondary } from 'design';
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

import type { MemberSelection } from 'e-teleport/AccessListManagement/Shared/Shared';
import {
  AccessListModified,
  EnrollingNestedListsAlert,
  getNewAndExistingUsersForAddingNewUsers,
} from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import {
  AccessList,
  AccessListMemberKind,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';

import { EnrollNewMembersFields } from '../Members/EnrollNewMembers';

type Props = {
  onClose(): void;
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
};

export function EnrollNewOwners({
  onClose,
  updateAccessList,
  accessList,
}: Props) {
  const { owners: existingOwners } = accessList;
  const { attempt, setAttempt } = useAttempt('');

  const [selectedOwners, setSelectedOwners] = useState<
    Option<MemberSelection>[]
  >([]);
  const [description, setDescription] = useState('');

  // duplicatedOwners are duplicate owners extracted from
  // selectedOwners.
  const [duplicatedOwners, setDuplicatedOwners] = useState<string[]>([]);

  const selectedOwnersContainAccessLists = useMemo(
    () =>
      selectedOwners.some(
        o => o.value?.membershipKind === AccessListMemberKind.List
      ),
    [selectedOwners]
  );

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
              name:
                typeof o.value === 'object' && 'name' in o.value
                  ? o.value.name
                  : o.value,
              membershipKind:
                typeof o.value === 'object' && 'membershipKind' in o.value
                  ? o.value.membershipKind
                  : AccessListMemberKind.User,
              description,
            })),
          ],
        },
        original: accessList,
      })
      .then(resp => {
        onClose();
        updateAccessList(resp);
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
            <DialogTitle>Enroll New Owners or Access Lists</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger">{attempt.statusText}</Alert>
            )}
            {duplicatedOwners.length > 0 && (
              <Alert kind="danger">
                {`The following usernames are already enrolled, remove them from the list to continue: ${duplicatedOwners.join(', ')}`}
              </Alert>
            )}
            <EnrollNewMembersFields
              selectedMembers={selectedOwners}
              setSelectedMembers={setSelectedOwners}
              attempt={attempt}
            />
            {selectedOwnersContainAccessLists && (
              <EnrollingNestedListsAlert
                kind="owner"
                listName={accessList.title}
              />
            )}
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
