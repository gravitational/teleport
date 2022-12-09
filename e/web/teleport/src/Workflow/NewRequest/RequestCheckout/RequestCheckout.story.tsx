import React, { useState } from 'react';
import { MemoryRouter } from 'react-router-dom';

import {
  RequestCheckout,
  RequestCheckoutProps,
  SuccessActionComponent,
} from './RequestCheckout';

export default {
  title: 'TeleportE/Workflow/Checkout',
};

export const Loaded = () => <RequestCheckout {...props} />;
export const Empty = () => <RequestCheckout {...props} data={[]} />;

export const Failed = () => (
  <RequestCheckout
    {...props}
    requireReason={false}
    createAttempt={{ status: 'failed', statusText: 'some error message' }}
    SuccessComponent={SuccessActionComponent}
  />
);

export const LoadedResourceRequest = () => {
  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState(props.resourceRequestRoles);
  return (
    <RequestCheckout
      {...props}
      isResourceRequest={true}
      fetchResourceRequestRolesAttempt={{ status: 'success' }}
      selectedResourceRequestRoles={selectedResourceRequestRoles}
      setSelectedResourceRequestRoles={setSelectedResourceRequestRoles}
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
  createRequest: () => null,
  data: [
    { kind: 'app', name: 'app-name', id: 'app-name' },
    { kind: 'db', name: 'db-name', id: 'app-name' },
    { kind: 'kube_cluster', name: 'kube-name', id: 'app-name' },
    { kind: 'windows_desktop', name: 'desktop-name', id: 'app-name' },
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
};
