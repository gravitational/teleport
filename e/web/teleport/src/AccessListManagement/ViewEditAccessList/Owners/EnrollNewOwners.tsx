import { useEffect, useMemo, useState } from 'react';
import { Alert, ButtonPrimary, ButtonSecondary } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Validation, { Validator } from 'shared/components/Validation';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { Option } from 'shared/components/Select';

import {
  AccessList,
  AccessListMemberKind,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { EligibleUsersFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import {
  convertAccessListsToUserOptions,
  EnrollingNestedListsAlert,
  filterExistingUsersAndConvertToOption as filterOutExistingUsersAndConvertToOptionType,
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
} from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';

import type { AccessListModified } from '../Shared';
import type {
  MemberSelection,
  UserOption,
} from 'e-teleport/AccessListManagement/Shared/Shared';

type Props = {
  onClose(): void;
  userOptions: UserOption[];
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
  accessLists: AccessList[];
};

export function EnrollNewOwners({
  onClose,
  userOptions,
  updateAccessList,
  accessList,
  accessLists,
}: Props) {
  const { id, owners: existingOwners, ownershipRequires } = accessList;
  const { attempt, setAttempt } = useAttempt('');

  const [eligibleUsers, setEligibleUsers] = useState<Option<MemberSelection>[]>(
    []
  );
  const [selectedOwners, setSelectedOwners] = useState<
    Option<MemberSelection>[]
  >([]);
  const [description, setDescription] = useState('');

  // duplicatedOwners are duplicate owners extracted from
  // selectedOwners.
  const [duplicatedOwners, setDuplicatedOwners] = useState<string[]>([]);

  useEffect(() => {
    let filteredOwners: Option<MemberSelection>[] = [];

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

    filteredOwners = filteredOwners.concat(
      convertAccessListsToUserOptions(id, accessLists, existingOwners)
    );

    setEligibleUsers(filteredOwners);
  }, []);

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
