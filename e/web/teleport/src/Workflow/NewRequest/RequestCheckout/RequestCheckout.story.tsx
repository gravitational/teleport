import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { RequestCheckout, RequestCheckoutProps } from './RequestCheckout';

export default {
  title: 'TeleportE/Workflow/Checkout',
};

export const Loaded = () => <RequestCheckout {...props} />;
export const Empty = () => <RequestCheckout {...props} data={[]} />;

export const Failed = () => (
  <RequestCheckout
    {...props}
    requireReason={false}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

export const Success = () => (
  <MemoryRouter initialEntries={['']}>
    <RequestCheckout
      {...props}
      requireReason={false}
      attempt={{ status: 'success' }}
    />
  </MemoryRouter>
);

const props: RequestCheckoutProps = {
  attempt: { status: '' },
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
};
