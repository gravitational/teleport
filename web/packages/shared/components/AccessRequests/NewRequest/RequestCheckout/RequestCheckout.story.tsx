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

import { useState } from 'react';
import { Link, MemoryRouter } from 'react-router-dom';

import { Box, ButtonPrimary, ButtonText } from 'design';
import { Option } from 'shared/components/Select';

import { dryRunResponse } from '../../fixtures';
import { useSpecifiableFields } from '../useSpecifiableFields';
import {
  RequestCheckoutWithSlider,
  RequestCheckoutWithSliderProps,
} from './RequestCheckout';

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
  const props = useSpecifiableFields();
  if (!props.dryRunResponse) {
    props.onDryRunChange(dryRunResponse);
  }

  return (
    <MemoryRouter>
      <RequestCheckoutWithSlider {...baseProps} {...props} />
    </MemoryRouter>
  );
};
export const Empty = () => {
  const [selectedReviewers, setSelectedReviewers] = useState([]);
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  const [requestTTL, setRequestTTL] = useState<Option<number>>();

  return (
    <MemoryRouter>
      <RequestCheckoutWithSlider
        {...baseProps}
        pendingAccessRequests={[]}
        selectedReviewers={selectedReviewers}
        setSelectedReviewers={setSelectedReviewers}
        maxDuration={maxDuration}
        onMaxDurationChange={setMaxDuration}
        pendingRequestTtl={requestTTL}
        setPendingRequestTtl={setRequestTTL}
      />
    </MemoryRouter>
  );
};

export const Failed = () => (
  <MemoryRouter>
    <RequestCheckoutWithSlider
      {...baseProps}
      requireReason={false}
      createAttempt={{
        status: 'failed',
        statusText: 'some error message',
      }}
      SuccessComponent={SuccessActionComponent}
      selectedReviewers={[]}
    />
  </MemoryRouter>
);

export const LoadedResourceRequest = () => {
  const [selectedReviewers, setSelectedReviewers] = useState(
    baseProps.selectedReviewers
  );
  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState(baseProps.resourceRequestRoles);
  return (
    <MemoryRouter>
      <RequestCheckoutWithSlider
        {...baseProps}
        isResourceRequest={true}
        fetchResourceRequestRolesAttempt={{ status: 'success' }}
        selectedResourceRequestRoles={selectedResourceRequestRoles}
        setSelectedResourceRequestRoles={setSelectedResourceRequestRoles}
        selectedReviewers={selectedReviewers}
        setSelectedReviewers={setSelectedReviewers}
      />
    </MemoryRouter>
  );
};

export const ProcessingResourceRequest = () => (
  <MemoryRouter>
    <RequestCheckoutWithSlider
      {...baseProps}
      isResourceRequest={true}
      fetchResourceRequestRolesAttempt={{ status: 'processing' }}
    />
  </MemoryRouter>
);

export const FailedResourceRequest = () => (
  <MemoryRouter>
    <RequestCheckoutWithSlider
      {...baseProps}
      isResourceRequest={true}
      fetchResourceRequestRolesAttempt={{
        status: 'failed',
        statusText: 'An error has occurred',
      }}
    />
  </MemoryRouter>
);

export const FailedUnsupportedKubeResourceKindWithTooltip = () => (
  <MemoryRouter>
    <RequestCheckoutWithSlider
      {...baseProps}
      isResourceRequest={true}
      fetchResourceRequestRolesAttempt={{
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting to some or all of the requested Kubernetes resources. allowed kinds for each requestable roles: test-role-1: [deployment], test-role-2: [pod secret configmap service serviceaccount kube_node persistentvolume persistentvolumeclaim deployment replicaset statefulset daemonset clusterrole kube_role clusterrolebinding rolebinding cronjob job certificatesigningrequest ingress]`,
      }}
    />
  </MemoryRouter>
);

export const FailedUnsupportedKubeResourceKindWithoutTooltip = () => (
  <MemoryRouter>
    <RequestCheckoutWithSlider
      {...baseProps}
      isResourceRequest={true}
      fetchResourceRequestRolesAttempt={{
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting to some or all of the requested Kubernetes resources. allowed kinds for each requestable roles: test-role-1: [deployment]`,
      }}
    />
  </MemoryRouter>
);

export const Success = () => (
  <MemoryRouter initialEntries={['']}>
    <RequestCheckoutWithSlider
      {...baseProps}
      requireReason={false}
      createAttempt={{ status: 'success' }}
      SuccessComponent={SuccessActionComponent}
    />
  </MemoryRouter>
);

const baseProps: RequestCheckoutWithSliderProps = {
  fetchKubeNamespaces: async () => [
    'namespace1',
    'namespace2',
    'namespace3',
    'namespace4',
  ],
  updateNamespacesForKubeCluster: () => null,
  createAttempt: { status: '' },
  fetchResourceRequestRolesAttempt: { status: '' },
  isResourceRequest: false,
  requireReason: true,
  selectedReviewers: [
    {
      value: 'george washington',
      label: 'george washington',
      isSelected: true,
    },
  ],
  setSelectedReviewers: () => null,
  createRequest: () => null,
  pendingAccessRequests: [
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
    {
      kind: 'saml_idp_service_provider',
      name: 'app-saml',
      id: 'app-name',
    },
    {
      kind: 'aws_ic_account_assignment',
      name: 'account1',
      id: 'admin-on-account1',
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
  onMaxDurationChange: () => null,
  maxDurationOptions: [],
  pendingRequestTtl: { value: 0, label: '1 hour' },
  setPendingRequestTtl: () => null,
  pendingRequestTtlOptions: [],
  dryRunResponse,
  startTime: null,
  onStartTimeChange: () => null,
};
