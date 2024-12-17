import { useState, useEffect, useMemo } from 'react';
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

import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import {
  EditKind,
  fetchAndProcessSelectedRoles,
  rolesContainDenyRules,
} from 'e-teleport/AccessListManagement/Shared/Shared';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import {
  TraitConvenience,
  TraitLabel,
  TraitsCreator,
  convertTraitLabelsToAllUserTraits,
} from 'e-teleport/AccessListManagement/Traits';

import type { AccessListModified } from '../Shared';
import type { Role } from 'teleport/services/resources';

type Props = {
  onClose(): void;
  editKind: EditKind;
  fetchRoleOptions(input: string): Promise<Option[]>;
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
};

export function EditEligibilityOrGrantRoles({
  accessList,
  onClose,
  editKind,
  fetchRoleOptions,
  updateAccessList,
}: Props) {
  let existingRoles: string[] = accessList.grants.roles;
  let trait: TraitConvenience = accessList.grants;
  if (editKind === 'Member') {
    existingRoles = accessList.membershipRequires.roles;
    trait = accessList.membershipRequires;
  } else if (editKind === 'Owner') {
    existingRoles = accessList.ownershipRequires.roles;
    trait = accessList.ownershipRequires;
  } else if (editKind === 'OwnerGrants') {
    existingRoles = accessList.ownerGrants.roles;
    trait = accessList.ownerGrants;
  }
  const { attempt, setAttempt } = useAttempt('');
  const [traitLabels, setTraitLabels] = useState<TraitLabel[]>(
    trait.traitLabels
  );
  const [selectedRoles, setSelectedRoles] = useState<Option[]>([]);
  const processedRolesFetchAttempt = useAttempt('');
  const [processedRoles, setProcessedRoles] = useState<Role[]>([]);

  useEffect(() => {
    let selectedRoles = existingRoles.map(r => ({
      value: r,
      label: r,
    }));

    setSelectedRoles(selectedRoles);
  }, []);

  useEffect(() => {
    if (
      !['Grants', 'OwnerGrants'].includes(editKind) ||
      processedRolesFetchAttempt.attempt.status === 'processing'
    ) {
      return;
    }
    if (!selectedRoles?.length) {
      setProcessedRoles([]);
      return;
    }

    fetchAndProcessSelectedRoles(processedRolesFetchAttempt, selectedRoles)
      .then(setProcessedRoles)
      .catch(() => setProcessedRoles([]));
  }, [selectedRoles]);

  const selectedRolesContainDenyRules = useMemo(
    () => rolesContainDenyRules(processedRoles),
    [processedRoles]
  );

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    const roles = selectedRoles.map(r => r.value);
    const traits = convertTraitLabelsToAllUserTraits(traitLabels);
    let req: Partial<AccessList> = {
      ownershipRequires: { roles, traits },
    };
    if (editKind === 'Member') {
      req = {
        membershipRequires: { roles, traits },
      };
    } else if (editKind === 'Grants') {
      req = {
        grants: { roles, traits },
      };
    } else if (editKind === 'OwnerGrants') {
      req = {
        ownerGrants: { roles, traits },
      };
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList({ req, original: accessList })
      .then(resp => {
        onClose();
        updateAccessList(resp);
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  let dialogTitle = 'Edit Granted Member Permissions';
  let editBtnTitle = 'Save Permissions';

  if (editKind === 'OwnerGrants') {
    dialogTitle = 'Edit Granted Owner Permissions';
  } else if (editKind === 'Member' || editKind === 'Owner') {
    dialogTitle = `Edit ${editKind} Eligibility`;
    editBtnTitle = `Save Eligibility`;
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
            {attempt.status === 'failed' ? (
              <Alert kind="danger" children={attempt.statusText} />
            ) : selectedRolesContainDenyRules ? (
              <Alert kind="warning" children={selectedRolesContainDenyRules} />
            ) : null}
            <EligibilityOrGrantRolesFieldSelectAndCreate
              loadOptions={fetchRoleOptions}
              isDisabled={attempt.status === 'processing'}
              onChange={(vals: Option[]) => setSelectedRoles(vals || [])}
              selected={selectedRoles}
              autoFocus={true}
              editKind={editKind}
              optional={true}
            />
            <Box mt={2}>
              <TraitsCreator
                kind={editKind}
                traitLabels={traitLabels}
                isDisabled={attempt.status === 'processing'}
                updateTraitLabels={(traitLabels: TraitLabel[]) =>
                  setTraitLabels(traitLabels)
                }
              />
            </Box>
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
