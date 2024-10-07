import styled from 'styled-components';
import { Label } from 'design';
import { Option } from 'shared/components/Select';
import { AllUserTraits, User } from 'teleport/services/user';

import { Theme } from 'design/theme/themes/types';

// HybridUserOption
//
// An Option can be type User if a user selected
// an option from the dropdown options. This
// is only possible if the user had "user" rbac
// and was able to fetch users from the back.
//
// An Option can be type String if a user manually
// entered a user. This is allowed because
// not all users will have the rbac to
// list users and SSO users may not necessarily
// exist in the backend yet.
export type HybridUserOption = Option<User | string>;
export type UserOption = Option<User>;
export type EditKind = 'Member' | 'Owner' | 'Grants' | 'OwnerGrants';

/**
 * Inverses the color scheme if required. Using the slightly muted foreground
 * as a background is a bit icky, but I see no better choice.
 */
const inverseLabel = ({
  theme,
  inverse,
}: {
  theme: Theme;
  inverse?: boolean;
}) =>
  inverse
    ? {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.text.slightlyMuted,
      }
    : {};

export const TruncatingLabel = styled(Label)<{ inverse?: boolean }>`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  max-width: 160px;
  ${inverseLabel}
`;

export function matchRoles(
  rolesRequiredToBeEligible: string[],
  userOptions: UserOption[]
): UserOption[] {
  return userOptions.filter(userOpt => {
    const currRolesAssigned = userOpt.value.roles;
    return rolesRequiredToBeEligible.every(requiredRole =>
      currRolesAssigned.includes(requiredRole)
    );
  });
}

export function matchTraits(
  traitsRequiredToBeEligible: AllUserTraits,
  userOptions: UserOption[]
): UserOption[] {
  const requiredTraitKeys = Object.keys(traitsRequiredToBeEligible);

  return userOptions.filter(userOpt => {
    const currTraits = userOpt.value.allTraits;
    return requiredTraitKeys.every(requiredTraitKey => {
      const matchRequiredVals = traitsRequiredToBeEligible[requiredTraitKey];
      return (
        currTraits[requiredTraitKey] &&
        matchRequiredVals.every(requiredVal =>
          currTraits[requiredTraitKey].some(currVal => requiredVal == currVal)
        )
      );
    });
  });
}
