import { AllUserTraits } from 'teleport/services/user';

export type AccessList = {
  id: string;
  title: string; // friendly name of id
  description?: string;
  audit: AccessListAudit;
  grants: AccessListGrant;
  // membershipRequires describes the requirements for a user to be a member of the access list.
  // For a membership to an access list to be effective, the user must meet the requirements of
  // membershipRequires and must be in the members list.
  membershipRequires?: AccessListRequires;
  members?: AccessListMember[];
  // membersCount is the total member we have in this access list.
  // membersCount only comes back when we "list" all access lists
  // in place of returning possible large amounts of AccessListMember[]
  // per access list.
  // Only owners and admins can view members.
  // Users who are members of this list will always get a 0 returned.
  membersCount?: number | undefined;
  // ownershipRequires describes the requirements for a user to be an owner of the access list.
  // For ownership of an access list to be effective, the user must meet the requirements of
  // ownershipRequires and must be in the owners list.
  ownershipRequires: AccessListRequires;
  owners: AccessListOwner[];
};

// A user must match both roles and traits to
// be found "eligible" (gets granted additional permissions)
export type AccessListRequires = {
  roles: string[];
  traits: AllUserTraits;
};

export type AccessListMember = {
  // name is the username of the member of the access list.
  name: string;
  // joined is when the user joined the access list.
  joined: Date;
  // expires is when the user's membership to the access list expires.
  expires?: Date;
  // reason is the reason this user was added to the access list.
  reason?: string;
  // addedBy is the user that added this user to the access list.
  addedBy: string;
  // ineligibleReason is a description on why this member
  // no longer meets requirements as defined in membershipRequires.
  ineligibleReason?: string; // TODO(lisa) need to come from the back
};

export type AccessListOwner = {
  // name is the username of the owner of the access list.
  name: string;
  // description is the plaintext description of the owner
  // and why they are an owner.
  description?: string;
  // ineligibleReason is a description on why this owner
  // no longer meets requirements as defined in ownershipRequires.
  ineligibleReason?: string; // TODO(lisa) need to come from the back
};

// AccessListAudit describes the frequency that this
// access list must be audited.
export type AccessListAudit = {
  frequency: string;
  nextDate: Date;
};

// AccessListGrant describes the access granted
// by membership to this access list.
export type AccessListGrant = {
  roles: string[];
  traits: AllUserTraits;
};

export type OwnerRequest = Omit<AccessListOwner, 'ineligibleReason'>;
export type MemberRequest = Omit<
  AccessListMember,
  'addedBy' | 'ineligibleReason'
> & {
  added_by: string;
};

export type UpsertAccessListRequest = {
  title: string;
  description?: string;
  grants: AccessListGrant;
  owners: OwnerRequest[];
  ownership_requires: AccessListRequires;
  membership_requires?: AccessListRequires;
  members?: MemberRequest[];
  audit: { frequency: string; next_audit_date: Date };
};
