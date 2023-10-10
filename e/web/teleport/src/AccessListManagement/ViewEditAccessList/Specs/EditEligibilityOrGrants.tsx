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

import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { EditKind } from 'e-teleport/AccessListManagement/Shared/Shared';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from 'e-teleport/AccessListManagement/CreateAccessList/Shared';
import {
  TraitConvenience,
  TraitLabel,
  TraitsCreator,
  convertTraitLabelsToAllUserTraits,
} from 'e-teleport/AccessListManagement/Traits';

import { AccessListModified } from '../ViewEditAccessList';

type Props = {
  onClose(): void;
  editKind: EditKind;
  roleOptions: Option[];
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
};

export function EditEligibilityOrGrantRoles({
  accessList,
  onClose,
  editKind,
  roleOptions,
  fetchAccessList,
}: Props) {
  let existingRoles: string[] = accessList.grants.roles;
  let trait: TraitConvenience = accessList.grants;
  if (editKind === 'Member') {
    existingRoles = accessList.membershipRequires.roles;
    trait = accessList.membershipRequires;
  } else if (editKind === 'Owner') {
    existingRoles = accessList.ownershipRequires.roles;
    trait = accessList.ownershipRequires;
  }
  const { attempt, setAttempt } = useAttempt('');
  const [traitLabels, setTraitLabels] = useState<TraitLabel[]>(
    trait.traitLabels
  );
  const [selectedRoles, setSelectedRoles] = useState<Option[]>([]);

  useEffect(() => {
    let selectedRoles = existingRoles.map(r => ({
      value: r,
      label: r,
    }));

    setSelectedRoles(selectedRoles);
  }, []);

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
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList({ req, original: accessList })
      .then(() => {
        onClose();
        fetchAccessList();
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  let dialogTitle = 'Edit Permissions Granted';
  let editBtnTitle = 'Edit Permissions Granted';

  if (editKind !== 'Grants') {
    dialogTitle = `Edit ${editKind} Eligibility`;
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
