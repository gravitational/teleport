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

import type { Meta, StoryObj } from '@storybook/react-vite';
import { action } from 'storybook/actions';

import {
  Attempt,
  makeEmptyAttempt,
  makeErrorAttempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';
import { AccessRequest, RequestKind } from 'shared/services/accessRequests';

import {
  requestResourceApprovedWithConstraints,
  requestResourcePendingWithConstraints,
  requestResourceWithConstraintsSuggestedAccessLists,
  requestRoleApproved,
  requestRoleApprovedWithStartTime,
  requestRoleDenied,
  requestRoleEmpty,
  requestRolePending,
  requestRolePromoted,
  requestSearchPending,
} from '../../fixtures';
import { RequestView, RequestViewProps } from './RequestView';
import { SuggestedAccessList } from './types';

type RequestState =
  | 'searchPending'
  | 'rolePending'
  | 'roleDenied'
  | 'roleApproved'
  | 'roleApprovedWithStartTime'
  | 'resourcePendingWithConstraints'
  | 'resourceApprovedWithConstraints'
  | 'rolePromoted'
  | 'roleEmpty'
  | 'shortTermResource'
  | 'longTermResource'
  | 'processing'
  | 'failed';

type SuggestionsState =
  | 'empty'
  | 'lists'
  | 'constraintLists'
  | 'permissionDenied'
  | 'error'
  | 'processing';

type StoryArgs = {
  request: RequestState;
  suggestions: SuggestionsState;
  canReview: boolean;
  canDelete: boolean;
  canAssume: boolean;
  isAssumed: boolean;
  isPromoted: boolean;
  ownRequest: boolean;
};

function requestAttempt(r: RequestState): Attempt<AccessRequest> {
  switch (r) {
    case 'processing':
      return makeProcessingAttempt();
    case 'failed':
      return makeErrorAttempt(new Error('some error message'));
    case 'searchPending':
      return makeSuccessAttempt(requestSearchPending);
    case 'rolePending':
      return makeSuccessAttempt(requestRolePending);
    case 'roleDenied':
      return makeSuccessAttempt(requestRoleDenied);
    case 'roleApproved':
      return makeSuccessAttempt(requestRoleApproved);
    case 'roleApprovedWithStartTime':
      return makeSuccessAttempt(requestRoleApprovedWithStartTime);
    case 'resourcePendingWithConstraints':
      return makeSuccessAttempt(requestResourcePendingWithConstraints);
    case 'resourceApprovedWithConstraints':
      return makeSuccessAttempt(requestResourceApprovedWithConstraints);
    case 'rolePromoted':
      return makeSuccessAttempt(requestRolePromoted);
    case 'roleEmpty':
      return makeSuccessAttempt(requestRoleEmpty);
    case 'shortTermResource':
      return makeSuccessAttempt({
        ...requestResourcePendingWithConstraints,
        requestKind: RequestKind.ShortTerm,
      });
    case 'longTermResource':
      return makeSuccessAttempt({
        ...requestResourcePendingWithConstraints,
        requestKind: RequestKind.LongTerm,
      });
  }
}

function suggestionsAttempt(
  s: SuggestionsState
): Attempt<SuggestedAccessList[]> {
  switch (s) {
    case 'empty':
      return makeSuccessAttempt([]);
    case 'lists':
      return makeSuccessAttempt(suggestedAccessLists);
    case 'constraintLists':
      return makeSuccessAttempt(
        requestResourceWithConstraintsSuggestedAccessLists
      );
    case 'permissionDenied':
      // Mirrors a backend RBAC error
      return makeErrorAttempt(
        Object.assign(
          new Error('access denied to perform action "read" on access list'),
          { response: { status: 403 } }
        )
      );
    case 'error':
      return makeErrorAttempt(
        new Error('some kind of error came back from the backend')
      );
    case 'processing':
      return makeProcessingAttempt();
  }
}

const meta = {
  title: 'Shared/AccessRequests/RequestView',
  argTypes: {
    request: {
      control: 'select',
      options: [
        'searchPending',
        'rolePending',
        'roleDenied',
        'roleApproved',
        'roleApprovedWithStartTime',
        'resourcePendingWithConstraints',
        'resourceApprovedWithConstraints',
        'rolePromoted',
        'roleEmpty',
        'shortTermResource',
        'longTermResource',
        'processing',
        'failed',
      ],
      description:
        'The request being reviewed. Long-term resource requests hide short-term approval, leaving only Access List promotion and reject.',
    },
    suggestions: {
      control: 'select',
      options: [
        'empty',
        'lists',
        'constraintLists',
        'permissionDenied',
        'error',
        'processing',
      ],
      description:
        'State of the suggested Access Lists fetch that gates the long-term promote option.',
    },
    canReview: { control: 'boolean' },
    canDelete: { control: 'boolean' },
    canAssume: { control: 'boolean' },
    isAssumed: { control: 'boolean' },
    isPromoted: { control: 'boolean' },
    ownRequest: { control: 'boolean' },
  },
  args: {
    request: 'rolePending',
    suggestions: 'empty',
    canReview: false,
    canDelete: false,
    canAssume: false,
    isAssumed: false,
    isPromoted: false,
    ownRequest: false,
  },
  render: (args: StoryArgs) => {
    const props: RequestViewProps = {
      user: 'loggedInUsername',
      fetchRequestAttempt: requestAttempt(args.request),
      submitReviewAttempt: makeEmptyAttempt(),
      getFlags: () => ({
        canReview: args.canReview,
        canDelete: args.canDelete,
        canAssume: args.canAssume,
        isAssumed: args.isAssumed,
        isPromoted: args.isPromoted,
        ownRequest: args.ownRequest,
      }),
      confirmDelete: false,
      toggleConfirmDelete: action('toggleConfirmDelete'),
      submitReview: action('submitReview'),
      assumeRole: action('assumeRole'),
      fetchSuggestedAccessListsAttempt: suggestionsAttempt(args.suggestions),
      assumeRoleAttempt: makeEmptyAttempt(),
      assumeAccessList: action('assumeAccessList'),
      deleteRequestAttempt: makeEmptyAttempt(),
      deleteRequest: action('deleteRequest'),
    };
    return (
      <RequestView key={`${args.request}-${args.suggestions}`} {...props} />
    );
  },
} satisfies Meta<StoryArgs>;

export default meta;

type Story = StoryObj<StoryArgs>;

export const LoadedSearchPending: Story = {
  args: { request: 'searchPending', canReview: true, canDelete: true },
};

export const LoadedRolePending: Story = {
  args: { request: 'rolePending', canReview: true, canDelete: true },
};

export const LoadedRoleDenied: Story = {
  args: { request: 'roleDenied', canDelete: true },
};

export const LoadedRoleApproved: Story = {
  args: { request: 'roleApproved', canDelete: true, canAssume: true },
};

export const LoadedRoleApprovedWithStartTime: Story = {
  args: { request: 'roleApprovedWithStartTime', canAssume: true },
};

export const LoadedResourcePendingWithConstraints: Story = {
  args: {
    request: 'resourcePendingWithConstraints',
    suggestions: 'constraintLists',
    canReview: true,
    canDelete: true,
  },
};

export const LoadedResourceApprovedWithConstraints: Story = {
  args: { request: 'resourceApprovedWithConstraints', canAssume: true },
};

export const AccessListPromoted: Story = {
  args: { request: 'rolePromoted', suggestions: 'lists', isPromoted: true },
};

export const AccessListPromotedOwnRequest: Story = {
  args: {
    request: 'rolePromoted',
    suggestions: 'lists',
    isPromoted: true,
    ownRequest: true,
  },
};

export const AccessListPending: Story = {
  args: { request: 'rolePending', suggestions: 'lists', canReview: true },
};

export const AccessListPendingWithError: Story = {
  args: { request: 'rolePending', suggestions: 'error', canReview: true },
};

export const LongTermWithSuggestedAccessLists: Story = {
  args: {
    request: 'longTermResource',
    suggestions: 'constraintLists',
    canReview: true,
  },
};

export const LongTermNoEligibleAccessList: Story = {
  args: {
    request: 'longTermResource',
    suggestions: 'empty',
    canReview: true,
  },
};

export const LongTermSuggestionsPermissionDenied: Story = {
  args: {
    request: 'longTermResource',
    suggestions: 'permissionDenied',
    canReview: true,
  },
};

export const LongTermSuggestionsError: Story = {
  args: {
    request: 'longTermResource',
    suggestions: 'error',
    canReview: true,
  },
};

export const LoadedEmpty: Story = {
  args: { request: 'roleEmpty', canAssume: true, isAssumed: true },
};

export const Processing: Story = {
  args: { request: 'processing' },
};

export const Failed: Story = {
  args: { request: 'failed' },
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
