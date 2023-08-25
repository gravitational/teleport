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
  // ownershipRequires describes the requirements for a user to be an owner of the access list.
  // For ownership of an access list to be effective, the user must meet the requirements of
  // ownershipRequires and must be in the owners list.
  ownershipRequires: AccessListRequires;
  owners: AccessListOwner[];
};

export type AccessListRequires = {
  // roles are the user roles that must be present
  // for the user to obtain access.
  roles: string[];
};

export type AccessListMember = {
  // name is the username of the member of the access list.
  name: string;
  // joined is when the user joined the access list.
  joined?: Date;
  // expires is when the user's membership to the access list expires.
  expires?: Date;
  // reason is the reason this user was added to the access list.
  reason?: string;
  // addedBy is the user that added this user to the access list.
  addedBy: string;
  // ineligibleReason is a description for the web UI only
  // on why this member is no longer eligible.
  ineligibleReason?: string;
};

export type AccessListOwner = {
  // name is the username of the owner of the access list.
  name: string;
  // description is the plaintext description of the owner
  // and why they are an owner.
  description?: string;
  // ineligibleReason is a description for the web UI only
  // on why this owner is no longer eligible.
  ineligibleReason?: string;
};

// AccessListAudit describes the frequency that this
// access list must be audited.
export type AccessListAudit = {
  frequency: string;
};

// AccessListGrant describes the access granted
// by membership to this access list.
export type AccessListGrant = {
  roles: string[];
};

type MembersRequest = Omit<AccessListMember, 'addedBy'> & { added_by: string };
export type CreateAccessListRequest = {
  // auditDuration should be in the format:
  // <number>h<number>m<number>s eg: 10h10m10s
  auditDuration: string;
  title: string;
  description?: string;
  grants: AccessListGrant;
  owners: AccessListOwner[];
  ownership_requires: AccessListRequires;
  membership_requires?: AccessListRequires;
  members?: MembersRequest[];
};

export type UpdateAccessListRequest = {
  members: string[]; // either add or delete
  owners: string[]; // either add or delete
};
