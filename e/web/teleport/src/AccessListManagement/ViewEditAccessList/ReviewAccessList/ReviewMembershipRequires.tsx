import { Box, H2 } from 'design';
import { Option } from 'shared/components/Select';

import { AccessListRequires } from 'e-teleport/services/accessmanagement';

import { EligibilityOrGrantRolesFieldSelectAndCreate } from '../../CreateAccessList/Shared';
import { TraitConvenience, TraitLabel, TraitsCreator } from '../../Traits';

export type MembershipRequires = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListRequires, 'traits'>;

type Props = {
  editedMembershipRequires: MembershipRequires;
  setEditedMembershipRequires(g: MembershipRequires): void;
};

export function ReviewMembershipRequires({
  editedMembershipRequires,
  setEditedMembershipRequires,
}: Props) {
  return (
    <>
      <H2 mb={3}>Membership Requirements</H2>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        isDisabled={false}
        onChange={(vals: Option[]) =>
          setEditedMembershipRequires({
            ...editedMembershipRequires,
            roles: vals ? vals.map(o => o.value) : [],
          })
        }
        selected={editedMembershipRequires.roles.map(r => ({
          value: r,
          label: r,
        }))}
        autoFocus={true}
        userKind="Members"
        rolesSelectedFor="eligibility"
        optional={true}
      />
      <Box mt={2}>
        <TraitsCreator
          kind={'Member'}
          traitLabels={editedMembershipRequires.traitLabels}
          isDisabled={false}
          updateTraitLabels={(traitLabels: TraitLabel[]) =>
            setEditedMembershipRequires({
              ...editedMembershipRequires,
              traitLabels,
            })
          }
        />
      </Box>
    </>
  );
}
