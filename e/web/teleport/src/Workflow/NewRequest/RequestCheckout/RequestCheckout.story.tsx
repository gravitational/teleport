import React, { useState } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { Option } from 'shared/components/Select';

import { dryRunResponse } from 'e-teleport/Workflow/fixtures';

import {
  RequestCheckout,
  RequestCheckoutProps,
  SuccessActionComponent,
} from './RequestCheckout';

export default {
  title: 'TeleportE/Workflow/Checkout',
};

export const Loaded = () => {
  const [selectedReviewers, setSelectedReviewers] = useState(
    props.selectedReviewers
  );
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

  return (
    <RequestCheckout
      {...props}
      selectedReviewers={selectedReviewers}
      setSelectedReviewers={setSelectedReviewers}
      maxDuration={maxDuration}
      setMaxDuration={setMaxDuration}
    />
  );
};
export const Empty = () => {
  const [selectedReviewers, setSelectedReviewers] = useState([]);
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

  return (
    <RequestCheckout
      {...props}
      data={[]}
      selectedReviewers={selectedReviewers}
      setSelectedReviewers={setSelectedReviewers}
      maxDuration={maxDuration}
      setMaxDuration={setMaxDuration}
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
  requestTTLDurationOptions: [{ value: 0, label: '' }],
  requestTTL: { value: 0, label: '1 hour' },
  setRequestTTL: () => null,
  dryRunResponse,
};
