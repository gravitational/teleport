/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';

import {
  makeSuccessAttempt,
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
} from 'shared/hooks/useAsync';

import {
  requestRoleApproved,
  requestRoleDenied,
  requestRolePending,
  requestSearchPending,
  requestRoleEmpty,
  requestRolePromoted,
  requestRoleApprovedWithStartTime,
} from '../../fixtures';

import { RequestView, RequestViewProps } from './RequestView';
import { RequestFlags, SuggestedAccessList } from './types';

export default {
  title: 'Shared/AccessRequests/RequestView',
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

export const LoadedRoleApprovedWithStartTime = () => {
  const flags = {
    ...sampleFlags,
    canAssume: true,
  };
  return (
    <RequestView
      {...sample}
      fetchRequestAttempt={makeSuccessAttempt(requestRoleApprovedWithStartTime)}
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
  assumeAccessList: () => null,
  deleteRequestAttempt: makeEmptyAttempt(),
  deleteRequest: () => null,
};

const sampleFlags: RequestFlags = {
  canAssume: false,
  isAssumed: false,
  canDelete: false,
  canReview: false,
  ownRequest: false,
  isPromoted: false,
};

const suggestedAccessLists: SuggestedAccessList[] = [
  {
    id: 'id-123456',
    title: 'Design Team',
    description: 'some description about this design team access list',
    grants: {
      roles: ['access', 'editor'],
      traits: { fruit: ['apple'], drink: ['mocha', 'latte', 'capppuccino'] },
    },
  },
  {
    id: 'id-9876',
    title: 'Managers',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat',
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
  },
];
