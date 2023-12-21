import React from 'react';

import {
  makeSuccessAttempt,
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
} from 'shared/hooks/useAsync';

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

import { RequestView, RequestViewProps } from './RequestView';
import { RequestFlags } from './types';

export default {
  title: 'TeleportE/Workflow/RequestView',
};

export const LoadedSearchPending = () => {
  const flags = {
    ...sampleFlags,
    canReview: true,
    canDelete: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestSearchPending)}
      getFlags={() => flags}
    />
  );
};

export const LoadedRolePending = () => {
  const flags = {
    ...sampleFlags,
    canReview: true,
    canDelete: true,
  };
  return <RequestView {...sample} getFlags={() => flags} />;
};

export const LoadedRoleDenied = () => {
  const flags = {
    ...sampleFlags,
    canDelete: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestRoleDenied)}
      getFlags={() => flags}
    />
  );
};

export const LoadedRoleApproved = () => {
  const flags = {
    ...sampleFlags,
    canDelete: true,
    canAssume: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestRoleApproved)}
      getFlags={() => flags}
    />
  );
};

export const AccessListPromoted = () => {
  const flags = {
    ...sampleFlags,
    isPromoted: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestRolePromoted)}
      getFlags={() => flags}
      fetchSuggestedAccessListsAttempt={makeSuccessAttempt(
        suggestedAccessLists
      )}
    />
  );
};

export const AccessListPromotedOwnRequest = () => {
  const flags = {
    ...sampleFlags,
    isPromoted: true,
    ownRequest: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestRolePromoted)}
      getFlags={() => flags}
      fetchSuggestedAccessListsAttempt={makeSuccessAttempt(
        suggestedAccessLists
      )}
    />
  );
};

export const AccessListPending = () => {
  const flags = {
    ...sampleFlags,
    canReview: true,
  };
  return (
    <RequestView
      {...sample}
      getFlags={() => flags}
      fetchSuggestedAccessListsAttempt={makeSuccessAttempt(
        suggestedAccessLists
      )}
    />
  );
};

export const AccessListPendingWithError = () => {
  const flags = {
    ...sampleFlags,
    canReview: true,
  };
  return (
    <RequestView
      {...sample}
      getFlags={() => flags}
      fetchSuggestedAccessListsAttempt={makeErrorAttempt(
        new Error('some kind of error came back from the backend')
      )}
    />
  );
};

export const LoadedEmpty = () => {
  const flags = {
    ...sampleFlags,
    canAssume: true,
    isAssumed: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestRoleEmpty)}
      getFlags={() => flags}
    />
  );
};

export const Processing = () => {
  return (
    <RequestView {...sample} fetchRequestAttempt={makeProcessingAttempt()} />
  );
};

export const Failed = () => {
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeErrorAttempt(new Error('some error message'))}
    />
  );
};

const sample: RequestViewProps = {
  user: 'loggedInUsername',
  fetchRequestAttempt: makeSuccessAttempt(requestRolePending),
  submitReviewAttempt: makeEmptyAttempt(),
  getFlags: () => sampleFlags,
  confirmDelete: false,
  toggleConfirmDelete: () => null,
  submitReview: () => null,
  assumeRole: () => null,
  fetchSuggestedAccessListsAttempt: makeSuccessAttempt([]),
  assumeRoleAttempt: makeEmptyAttempt(),
};

const sampleFlags: RequestFlags = {
  canAssume: false,
  isAssumed: false,
  canDelete: false,
  canReview: false,
  ownRequest: false,
  isPromoted: false,
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
