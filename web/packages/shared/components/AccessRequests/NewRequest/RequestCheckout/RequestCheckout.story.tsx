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

import React, { useState } from 'react';
import { MemoryRouter, Link } from 'react-router-dom';

import { Box, ButtonPrimary, ButtonText } from 'design';

import { Option } from 'shared/components/Select';

import { dryRunResponse } from '../../fixtures';

import { RequestCheckout, RequestCheckoutProps } from './RequestCheckout';

export default {
  title: 'Shared/AccessRequests/Checkout',
};

function SuccessActionComponent({ reset, onClose }) {
  return (
    <Box textAlign="center">
      <ButtonPrimary
        as={Link}
        mt={5}
        mb={3}
        width="100%"
        size="large"
        to={'/web/requests'}
      >
        Back to Listings
      </ButtonPrimary>
      <ButtonText
        onClick={() => {
          reset();
          onClose();
        }}
      >
        Make Another Request
      </ButtonText>
    </Box>
  );
}

export const Loaded = () => {
  const [selectedReviewers, setSelectedReviewers] = useState(
    props.selectedReviewers
  );
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  const [requestTTL, setRequestTTL] = useState<Option<number>>();

  return (
    <RequestCheckout
      {...props}
      selectedReviewers={selectedReviewers}
      setSelectedReviewers={setSelectedReviewers}
      maxDuration={maxDuration}
      setMaxDuration={setMaxDuration}
      requestTTL={requestTTL}
      setRequestTTL={setRequestTTL}
    />
  );
};
export const Empty = () => {
  const [selectedReviewers, setSelectedReviewers] = useState([]);
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  const [requestTTL, setRequestTTL] = useState<Option<number>>();

  return (
    <RequestCheckout
      {...props}
      data={[]}
      selectedReviewers={selectedReviewers}
      setSelectedReviewers={setSelectedReviewers}
      maxDuration={maxDuration}
      setMaxDuration={setMaxDuration}
      requestTTL={requestTTL}
      setRequestTTL={setRequestTTL}
    />
  );
};

export const Failed = () => (
  <RequestCheckout
    {...props}
    requireReason={false}
    createAttempt={{
      status: 'failed',
      statusText: 'some error message',
    }}
    SuccessComponent={SuccessActionComponent}
    selectedReviewers={[]}
  />
);

export const LoadedResourceRequest = () => {
  const [selectedReviewers, setSelectedReviewers] = useState(
    props.selectedReviewers
  );
  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState(props.resourceRequestRoles);
  return (
    <RequestCheckout
      {...props}
      isResourceRequest={true}
      fetchResourceRequestRolesAttempt={{ status: 'success' }}
      selectedResourceRequestRoles={selectedResourceRequestRoles}
      setSelectedResourceRequestRoles={setSelectedResourceRequestRoles}
      selectedReviewers={selectedReviewers}
      setSelectedReviewers={setSelectedReviewers}
    />
  );
};

export const ProcessingResourceRequest = () => (
  <RequestCheckout
    {...props}
    isResourceRequest={true}
    fetchResourceRequestRolesAttempt={{ status: 'processing' }}
  />
);

export const FailedResourceRequest = () => (
  <RequestCheckout
    {...props}
    isResourceRequest={true}
    fetchResourceRequestRolesAttempt={{
      status: 'failed',
      statusText: 'An error has occurred',
    }}
  />
);

export const Success = () => (
  <MemoryRouter initialEntries={['']}>
    <RequestCheckout
      {...props}
      requireReason={false}
      createAttempt={{ status: 'success' }}
      SuccessComponent={SuccessActionComponent}
    />
  </MemoryRouter>
);

const props: RequestCheckoutProps = {
  createAttempt: { status: '' },
  fetchResourceRequestRolesAttempt: { status: '' },
  isResourceRequest: false,
  requireReason: true,
  reviewers: ['bob', 'cat', 'george washington'],
  selectedReviewers: [
    { value: 'bob', label: 'bob', isSelected: true },
    { value: 'cat', label: 'cat', isSelected: true },
    {
      value: 'george washington',
      label: 'george washington',
      isSelected: true,
    },
  ],
  setSelectedReviewers: () => null,
  createRequest: () => null,
  data: [
    {
      kind: 'app',
      name: 'app-name',
      id: 'app-name',
    },
    {
      kind: 'db',
      name: 'app-name',
      id: 'app-name',
    },
    {
      kind: 'kube_cluster',
      name: 'kube-name',
      id: 'app-name',
    },
    {
      kind: 'user_group',
      name: 'user-group-name',
      id: 'app-name',
    },
    {
      kind: 'windows_desktop',
      name: 'desktop-name',
      id: 'app-name',
    },
  ],
  clearAttempt: () => null,
  onClose: () => null,
  toggleResource: () => null,
  reset: () => null,
  transitionState: 'entered',
  numRequestedResources: 4,
  resourceRequestRoles: ['admin', 'access', 'developer'],
  selectedResourceRequestRoles: ['admin', 'access'],
  setSelectedResourceRequestRoles: () => null,
  fetchStatus: 'loaded',
  maxDuration: { value: 0, label: '12 hours' },
  setMaxDuration: () => null,
  requestTTL: { value: 0, label: '1 hour' },
  setRequestTTL: () => null,
  dryRunResponse,
};
