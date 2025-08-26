import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

import { AllUserTraits } from 'teleport/services/user';

// IneligibleStatus describes a member or owner's
// ineligibility.
export enum IneligibleStatus {
  // User was checked and is eligible.
  Eligible = 'INELIGIBLE_STATUS_ELIGIBLE',
  // User does not exist in the backend.
  UserNotExist = 'INELIGIBLE_STATUS_USER_NOT_EXIST',
  // User does not meet the eligibility defined by
  // AccessListRequires fields.
  MissingRequirements = 'INELIGIBLE_STATUS_MISSING_REQUIREMENTS',
  // User is expired.
  // Only applies to members.
  Expired = 'INELIGIBLE_STATUS_EXPIRED',
}

// ReviewFrequency is the frequency of reviews.
export enum ReviewFrequency {
  OneMonth = '1 month',
  ThreeMonths = '3 months',
  SixMonths = '6 months', // default
  OneYear = '1 year',
}

export type ReviewFrequencyBackendParsableValue = '1m' | '3m' | '6m' | '12m';

// ReviewDayOfMonth is the day of month that reviews will repeat on.
export enum ReviewDayOfMonth {
  FirstDayOfMonth = '1', // default
  FifteenthDayOfMonth = '15',
  LastDayOfMonth = 'last',
}

export enum AccessListMemberKind {
  Unspecified = 'MEMBERSHIP_KIND_UNSPECIFIED',
  User = 'MEMBERSHIP_KIND_USER',
  List = 'MEMBERSHIP_KIND_LIST',
}

export type AccessListReview = {
  notes: string;
  reviewDate: Date;
  reviewers: string[];
  // Unprocessed data.
  // Used to display it as json for read only TextEditor.
  raw: any;
};

export type AccessListReviewResponse = {
  reviews: AccessListReview[];
  startKey: string;
};

export type AccessList = {
  id: string;
  title: string; // friendly name of id
  description?: string;
  origin?: AccessListOrigin;
  type: AccessListType;
  audit: AccessListAudit;
  grants: AccessListGrant; // memberGrant
  ownerGrants: AccessListGrant;
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
  // memberListsCount is the number of lists that are members of this access list.
  memberListCount?: number | undefined;
  // ownershipRequires describes the requirements for a user to be an owner of the access list.
  // For ownership of an access list to be effective, the user must meet the requirements of
  // ownershipRequires and must be in the owners list.
  ownershipRequires: AccessListRequires;
  owners: AccessListOwner[];
  // inheritedMemberGrants is the list of member grants by inheritance from parent access lists.
  inheritedMemberGrants: AccessListGrant;
  // currentUserAssignments describes the current user's membership and ownership in the access list.
  currentUserAssignments?: AccessListCurrentUserAssignments;
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
  ineligibleReason?: string;

  membershipKind: AccessListMemberKind;
};

export type AccessListOwner = {
  // name is the username of the owner of the access list.
  name: string;
  // description is the plaintext description of the owner
  // and why they are an owner.
  description?: string;
  // ineligibleReason is a description on why this owner
  // no longer meets requirements as defined in ownershipRequires.
  ineligibleReason?: string;

  membershipKind: AccessListMemberKind;
};

type AccessListAuditRecurrence = {
  frequency: ReviewFrequency;
  dayOfMonth: ReviewDayOfMonth;
};

// AccessListAudit describes the frequency that this
// access list must be audited.
export type AccessListAudit = {
  recurrence: AccessListAuditRecurrence;
  nextDate: Date;
};

// AccessListGrant describes the access granted
// by membership to this access list.
export type AccessListGrant = {
  roles: string[];
  traits: AllUserTraits;
};

// AccessListCurrentUserAssignments describes the current user's
// membership and ownership in a given access list.
export type AccessListCurrentUserAssignments = {
  ownershipType: AccessListUserAssignmentType;
  membershipType: AccessListUserAssignmentType;
};

export type OwnerRequest = Omit<
  AccessListOwner,
  'ineligibleReason' | 'membershipKind'
> & {
  membership_kind: AccessListMemberKind;
};
export type MemberRequest = Omit<
  AccessListMember,
  'addedBy' | 'ineligibleReason' | 'membershipKind'
> & {
  added_by: string;
  membership_kind: AccessListMemberKind;
};

export type UpsertAccessListRequest = {
  type: string;
  title: string;
  description?: string;
  grants: AccessListGrant;
  owners: OwnerRequest[];
  owner_grants: AccessListGrant;
  ownership_requires: AccessListRequires;
  membership_requires?: AccessListRequires;
  members?: MemberRequest[];
  audit: {
    recurrence: {
      day_of_month: ReviewDayOfMonth;
      frequency: ReviewFrequencyBackendParsableValue;
    };
    next_audit_date: Date;
  };
};

export type AddMembersToAccessListRequest = {
  members: MemberRequest[];
};

export type ReviewAccessListRequest = {
  name: string;
  reviewer: string;
  notes?: string;
  membershipRequires?: AccessListRequires;
  membersDeleted?: AccessListMember[];
  auditRecurrence?: AccessListAuditRecurrence;
};

/**
 * AccessListOrigin specifies the name of an integration
 * for which or from which the Access List was created.
 */
export enum AccessListOrigin {
  Unspecified = '',
  Okta = 'okta',
  AwsIdentityCenter = 'aws-identity-center',
}

export enum AccessListType {
  /**
   * Dynamic Access Lists are the default type supposed to be managed with the web UI. They
   * require periodic audit reviews.
   */
  Default = '',
  /**
   * Static Access Lists are supposed to be managed with the IaC tools like Terraform. Audit
   * reviews are not supported for them and the ownership is optional.
   */
  Static = 'static',
  /**
   * Scim Access Lists are created with the SCIM integration. Audit reviews are not supported
   * for them and the ownership is optional.
   */
  Scim = 'scim',
}

export function isReviewable(type: AccessListType): boolean {
  return type !== AccessListType.Scim && type !== AccessListType.Static;
}

export function isReadOnly(type: AccessListType): boolean {
  return type === AccessListType.Static;
}
