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
import { UNSUPPORTED_KINDS } from 'shared/components/AccessRequests/NewRequest/RequestCheckout/LongTerm';
import { Option } from 'shared/components/Select';
import { AccessRequest, RequestKind } from 'shared/services/accessRequests';

import { dryRunResponse } from '../../fixtures';
import { useSpecifiableFields } from '../useSpecifiableFields';
import {
  PendingListItem,
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

export const LoadedLongTermRequest = () => {
  const dryRunResponseWithLongTerm = {
    ...dryRunResponse,
    requestKind: RequestKind.LongTerm,
    longTermResourceGrouping: {
      accessListToResources: {
        'some-list-uuid': baseProps.pendingAccessRequests
          .filter(r => !UNSUPPORTED_KINDS.includes(r.kind))
          .map(r => ({
            kind: r.kind,
            name: r.id,
            clusterName: 'cluster-name',
          })),
      },
      canProceed: true,
      recommendedAccessList: 'some-list-uuid',
      validationMessage: '',
    },
  } satisfies AccessRequest;

  return (
    <MemoryRouter>
      <RequestCheckoutWithSlider
        {...baseProps}
        isResourceRequest={true}
        pendingAccessRequests={baseProps.pendingAccessRequests.filter(
          r => r.kind !== 'windows_desktop'
        )}
        fetchResourceRequestRolesAttempt={{ status: 'success' }}
        requestKind={RequestKind.LongTerm}
        dryRunResponse={dryRunResponseWithLongTerm}
      />
    </MemoryRouter>
  );
};

export const LoadedLongTermRequestWithGroupingErrors = () => {
  const dryRunResponseWithLongTermGroupingErrors = {
    ...dryRunResponse,
    requestKind: RequestKind.LongTerm,
    longTermResourceGrouping: {
      accessListToResources: {
        'some-list-uuid': baseProps.pendingAccessRequests
          .slice(0, 3)
          .map(r => ({
            kind: r.kind,
            name: r.id,
            clusterName: 'cluster-name',
          })),
        'another-list-uuid': baseProps.pendingAccessRequests
          .slice(3)
          .map(r => ({
            kind: r.kind,
            name: r.id,
            clusterName: 'cluster-name',
          })),
      },
      canProceed: false,
      recommendedAccessList: 'some-list-uuid',
      validationMessage:
        'Selected resources cannot be grouped for long-term access',
    },
  } satisfies AccessRequest;

  return (
    <MemoryRouter>
      <RequestCheckoutWithSlider
        {...baseProps}
        isResourceRequest={true}
        fetchResourceRequestRolesAttempt={{ status: 'success' }}
        requestKind={RequestKind.LongTerm}
        dryRunResponse={dryRunResponseWithLongTermGroupingErrors}
      />
    </MemoryRouter>
  );
};

export const LoadedLongTermRequestWithUnsupportedResources = () => {
  const unsupportedResources = [
    {
      kind: 'windows_desktop',
      name: 'desktop-name',
      id: 'desktop-id',
    },
    {
      kind: 'namespace',
      name: 'kube-name',
      id: 'kube-id',
      subResourceName: 'kube-namespace-name',
    },
  ] satisfies PendingListItem[];

  const dryRunResponseWithLongTermAndUnsupportedResources = {
    ...dryRunResponse,
    requestKind: RequestKind.LongTerm,
    resources: [
      ...baseProps.pendingAccessRequests.map(r => ({
        id: { ...r, clusterName: 'cluster-name' },
      })),
      ...unsupportedResources.map(r => ({
        id: { ...r, clusterName: 'cluster-name' },
      })),
    ],
    longTermResourceGrouping: {
      accessListToResources: {
        'list-uuid': baseProps.pendingAccessRequests
          .filter(r => !UNSUPPORTED_KINDS.includes(r.kind))
          .map(r => ({
            kind: r.kind,
            name: r.id,
            clusterName: 'cluster-name',
          })),
      },
      canProceed: false,
      recommendedAccessList: 'list-uuid',
      validationMessage:
        'Long-term access is not available for some selected resources',
    },
  } satisfies AccessRequest;

  return (
    <MemoryRouter>
      <RequestCheckoutWithSlider
        {...baseProps}
        isResourceRequest={true}
        pendingAccessRequests={[
          ...baseProps.pendingAccessRequests,
          ...unsupportedResources,
        ]}
        fetchResourceRequestRolesAttempt={{ status: 'success' }}
        requestKind={RequestKind.LongTerm}
        dryRunResponse={dryRunResponseWithLongTermAndUnsupportedResources}
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
  reasonPrompts: [],
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
      id: 'app-id',
    },
    {
      kind: 'db',
      name: 'db-name',
      id: 'db-id',
    },
    {
      kind: 'kube_cluster',
      name: 'kube-name',
      id: 'kube-id',
    },
    {
      kind: 'user_group',
      name: 'user-group-name',
      id: 'user-group-id',
    },
    {
      kind: 'windows_desktop',
      name: 'desktop-name',
      id: 'desktop-id',
    },
    {
      kind: 'saml_idp_service_provider',
      name: 'app-saml',
      id: 'saml-id',
    },
    {
      kind: 'aws_ic_account_assignment',
      name: 'account1',
      id: 'aws-id',
    },
  ],
  clearAttempt: () => null,
  onClose: () => null,
  toggleResource: () => null,
  toggleResources: () => null,
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
  requestKind: RequestKind.ShortTerm,
  setRequestKind: () => null,
  onStartTimeChange: () => null,
};
