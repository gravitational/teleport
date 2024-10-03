import React, { useState, PropsWithChildren } from 'react';
import styled from 'styled-components';
import { Popover, Text, Label } from 'design';
import { Option } from 'shared/components/Select';
import { AllUserTraits, User } from 'teleport/services/user';

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

// TODO(lisa): move this to 'shared/ToolTip' package
// and refactor ToolTipInfo with this.
export const ToolTipText: React.FC<
  PropsWithChildren<{
    tipContent: React.ReactElement;
    fontSize?: number;
  }>
> = ({ tipContent, fontSize = 10, children }) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <>
      <span
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
      >
        {children}
      </span>
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
      >
        <StyledOnHover px={2} py={1} fontSize={`${fontSize}px`}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

export const TruncatingLabel = styled(Label)`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  max-width: 160px;
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
