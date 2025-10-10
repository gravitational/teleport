import { Box, H2 } from 'design';
import { Option } from 'shared/components/Select';

import { AccessListGrant } from 'e-teleport/services/accessmanagement';

import { EligibilityOrGrantRolesFieldSelectAndCreate } from '../../CreateAccessList/Shared';
import { TraitConvenience, TraitLabel, TraitsCreator } from '../../Traits';

export type Grant = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListGrant, 'traits'>;

type Props = {
  editedGrants: Grant;
  setEditedGrants(g: Grant): void;
};

export function ReviewGrants({ editedGrants, setEditedGrants }: Props) {
  return (
    <>
      <H2 mb={3}>Permissions Granted to List Members</H2>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        isDisabled={false}
        onChange={(vals: Option[]) =>
          setEditedGrants({
            ...editedGrants,
            roles: vals ? vals.map(o => o.value) : [],
          })
        }
        selected={editedGrants.roles.map(r => ({ value: r, label: r }))}
        autoFocus={true}
        editKind="Grants"
        optional={editedGrants.traitLabels.length > 0}
      />
      <Box mt={2}>
        <TraitsCreator
          kind={'Grants'}
          traitLabels={editedGrants.traitLabels}
          isDisabled={false}
          updateTraitLabels={(traitLabels: TraitLabel[]) =>
            setEditedGrants({ ...editedGrants, traitLabels })
          }
        />
      </Box>
    </>
  );
}
