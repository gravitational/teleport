import React from 'react';

import {
  AccessList,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';

import {
  requestRoleApproved,
  requestRoleDenied,
  requestRolePending,
  requestSearchPending,
  requestRoleEmpty,
  requestRolePromoted,
} from '../../fixtures';

import { RequestView } from './RequestView';

export default {
  title: 'TeleportE/Workflow/RequestView',
};

export const LoadedSearchPending = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
    canDelete: true,
  };
  return (
    <RequestView {...sample} request={requestSearchPending} flags={flags} />
  );
};

export const LoadedRolePending = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
    canDelete: true,
  };
  return <RequestView {...sample} flags={flags} />;
};

export const LoadedRoleDenied = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
  };
  return <RequestView {...sample} request={requestRoleDenied} flags={flags} />;
};

export const LoadedRoleApproved = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
    canAssume: true,
  };
  return (
    <RequestView {...sample} request={requestRoleApproved} flags={flags} />
  );
};

export const AccessListPromoted = () => {
  const flags = {
    ...sample.flags,
    isPromoted: true,
  };
  return (
    <RequestView
      {...sample}
      request={requestRolePromoted}
      flags={flags}
      longTermAccess={{
        suggestedAccessLists,
        error: '',
      }}
    />
  );
};

export const AccessListPromotedOwnRequest = () => {
  const flags = {
    ...sample.flags,
    isPromoted: true,
    ownRequest: true,
  };
  return (
    <RequestView
      {...sample}
      request={requestRolePromoted}
      flags={flags}
      longTermAccess={{
        suggestedAccessLists,
        error: '',
      }}
    />
  );
};

export const AccessListPending = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
  };
  return (
    <RequestView
      {...sample}
      flags={flags}
      longTermAccess={{
        suggestedAccessLists,
        error: '',
      }}
    />
  );
};

export const AccessListPendingWithError = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
  };
  return (
    <RequestView
      {...sample}
      flags={flags}
      longTermAccess={{
        suggestedAccessLists: [],
        error: 'some kind of error came back from the backend',
      }}
    />
  );
};

export const LoadedEmpty = () => {
  const flags = {
    ...sample.flags,
    canAssume: true,
    isAssumed: true,
  };
  return <RequestView {...sample} request={requestRoleEmpty} flags={flags} />;
};

export const Processing = () => {
  return <RequestView {...sample} attempt={{ status: 'processing' }} />;
};

export const Failed = () => {
  return (
    <RequestView
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const sample = {
  user: 'loggedInUsername',
  attempt: { status: 'success' as any },
  reviewAttempt: { status: '' as any },
  request: requestRolePending,
  flags: {
    canAssume: false,
    isAssumed: false,
    canDelete: false,
    canReview: false,
    ownRequest: false,
    isPromoted: false,
  },
  confirmDelete: false,
  toggleConfirmDelete: () => null,
  submitReview: () => null,
  deleteRequest: () => null,
  assumeRole: () => null,
  longTermAccess: {
    suggestedAccessLists: [],
    error: '',
  },
};

const suggestedAccessLists: AccessList[] = [
  {
    id: 'id-123456',
    title: 'Design Team',
    description: 'some description about this design team access list',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneMonth,
        dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
      },
      nextDate: new Date('2023-08-24T17:48:15.78579Z'),
    },
    grants: {
      roles: ['access', 'editor'],
      traits: { fruit: ['apple'], drink: ['mocha', 'latte', 'capppuccino'] },
    },
    membershipRequires: {
      roles: ['intern'],
      traits: { fruit: ['banana'] },
    },
    members: [
      {
        name: 'george',
        joined: new Date(),
        expires: new Date(),
        reason: 'some reason',
        addedBy: 'llama',
        ineligibleReason: 'some member ineligible reason',
      },
    ],
    ownershipRequires: {
      roles: ['admin'],
      traits: { fruit: ['carrot'] },
    },
    owners: [
      {
        name: 'lisa',
        description: 'some description',
        ineligibleReason: 'some owner ineligible reason',
      },
    ],
  },
  {
    id: 'id-9876',
    title: 'Managers',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      },
      nextDate: new Date('2023-08-24T17:48:15.78579Z'),
    },
    grants: {
      roles: [
        'access',
        'devices',
        'editor',
        'devices',
        'reviewer',
        'auditor',
        'some really long role name goerge washington',
        'admin',
        'intern',
        'devices',
        'devices',
      ],
      traits: { fruit: ['apple'] },
    },
    membershipRequires: {
      roles: ['intern'],
      traits: { fruit: ['banana'] },
    },
    members: [
      {
        name: 'george',
        joined: new Date(),
        expires: new Date(),
        reason: 'some reason',
        addedBy: 'llama',
        ineligibleReason: 'some member ineligible reason',
      },
    ],
    ownershipRequires: {
      roles: ['admin'],
      traits: { fruit: ['carrot'] },
    },
    owners: [
      {
        name: 'lisa',
        description: 'some description',
        ineligibleReason: 'some owner ineligible reason',
      },
    ],
  },
];
