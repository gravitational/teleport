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

import React, {
  forwardRef,
  useEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
} from 'react';
import type { TransitionStatus } from 'react-transition-group';
import styled, { useTheme } from 'styled-components';

import {
  Alert,
  Box,
  ButtonBorder,
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  Link as ExternalLink,
  Flex,
  H2,
  Image,
  Indicator,
  LabelInput,
  P3,
  Subtitle2,
  Text,
  Toggle,
} from 'design';
import { Danger } from 'design/Alert';
import Table, { Cell } from 'design/DataTable';
import { ArrowBack, ChevronDown, ChevronRight, Warning } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { Option } from 'shared/components/Select';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { useRule, Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { mergeRefs } from 'shared/libs/mergeRefs';
import type {
  AccessRequest,
  LongTermResourceGrouping,
} from 'shared/services/accessRequests';
import { pluralize } from 'shared/utils/text';

import { AccessDurationRequest } from '../../AccessDuration';
import { AssumeStartTime } from '../../AssumeStartTime/AssumeStartTime';
import { CreateRequest } from '../../Shared/types';
import {
  checkSupportForKubeResources,
  isKubeClusterWithNamespaces,
} from '../kube';
import { AdditionalOptions } from './AdditionalOptions';
import { CrossIcon } from './CrossIcon';
import { KubeNamespaceSelector } from './KubeNamespaceSelector';
import { SelectReviewers } from './SelectReviewers';
import shieldCheck from './shield-check.png';
import { ReviewerOption } from './types';

export const RequestCheckoutWithSlider = forwardRef<
  HTMLDivElement,
  RequestCheckoutWithSliderProps<PendingListItem>
>(
  (
    { transitionState, ...props },
    /**
     * ref is extra ref that can be passed to RequestCheckoutWithSlider, at the moment used for
     * animations.
     */
    ref
  ) => {
    const wrapperRef = useRef<HTMLDivElement>(null);

    // Listeners are attached to enable overflow on the wrapper div after
    // transitioning ends (entered) or starts (exits). Enables vertical scrolling
    // when content gets too big.
    //
    // Overflow is initially hidden to prevent
    // brief flashing of horizontal scroll bar resulting from positioning
    // the container off screen to the right for the slide affect.
    useEffect(() => {
      function applyOverflowAutoStyle(e: TransitionEvent) {
        if (e.propertyName === 'right') {
          wrapperRef.current.style.overflow = `auto`;
          // There will only ever be one 'end right' transition invoked event, so we remove it
          // afterwards, and listen for the 'start right' transition which is only invoked
          // when user exits this component.
          window.removeEventListener('transitionend', applyOverflowAutoStyle);
          window.addEventListener('transitionstart', applyOverflowHiddenStyle);
        }
      }

      function applyOverflowHiddenStyle(e: TransitionEvent) {
        if (e.propertyName === 'right') {
          wrapperRef.current.style.overflow = `hidden`;
        }
      }

      window.addEventListener('transitionend', applyOverflowAutoStyle);

      return () => {
        window.removeEventListener('transitionend', applyOverflowAutoStyle);
        window.removeEventListener('transitionstart', applyOverflowHiddenStyle);
      };
    }, []);

    return (
      <div
        ref={mergeRefs([wrapperRef, ref])}
        data-testid="request-checkout"
        css={`
          position: absolute;
          width: 100vw;
          height: 100vh;
          top: 0;
          left: 0;
          overflow: hidden;
        `}
      >
        <Dimmer className={transitionState} />
        <SidePanel className={transitionState}>
          <RequestCheckout {...props} />
        </SidePanel>
      </div>
    );
  }
);

export function RequestCheckout<T extends PendingListItem>({
  toggleResource,
  toggleResources,
  onClose,
  reset,
  appsGrantedByUserGroup = [],
  userGroupFetchAttempt,
  clearAttempt,
  setSelectedReviewers,
  SuccessComponent,
  requireReason,
  reasonPrompts,
  numRequestedResources,
  setSelectedResourceRequestRoles,
  fetchStatus,
  onMaxDurationChange,
  maxDurationOptions,
  setPendingRequestTtl,
  pendingRequestTtlOptions,
  dryRunResponse,
  pendingAccessRequests,
  showClusterNameColumn,
  createAttempt,
  fetchResourceRequestRolesAttempt,
  createRequest,
  selectedReviewers,
  maxDuration,
  pendingRequestTtl,
  resourceRequestRoles,
  isResourceRequest,
  selectedResourceRequestRoles,
  Header,
  startTime,
  onStartTimeChange,
  fetchKubeNamespaces,
  updateNamespacesForKubeCluster,
  longTerm,
  setLongTerm,
}: RequestCheckoutProps<T>) {
  const theme = useTheme();
  const [reason, setReason] = useState('');

  function updateReason(reason: string) {
    setReason(reason);
  }

  function handleOnSubmit(validator: Validator) {
    if (
      !validator.validate() ||
      (isResourceRequest &&
        dryRunResponse?.longTermResourceGrouping &&
        !dryRunResponse.longTermResourceGrouping.canProceed)
    ) {
      return;
    }

    createRequest({
      reason,
      suggestedReviewers: selectedReviewers.map(r => r.value),
      maxDuration: maxDuration ? new Date(maxDuration.value) : null,
      requestTTL: pendingRequestTtl ? new Date(pendingRequestTtl.value) : null,
      start: startTime,
      longTerm,
    });
  }

  const { requestKubeResourceSupported, isRequestKubeResourceError } =
    checkSupportForKubeResources(fetchResourceRequestRolesAttempt);
  const hasUnsupporteKubeResourceKinds =
    !requestKubeResourceSupported && isRequestKubeResourceError;

  const isInvalidRoleSelection =
    resourceRequestRoles.length > 0 &&
    isResourceRequest &&
    selectedResourceRequestRoles.length < 1;

  const submitBtnDisabled = useMemo(() => {
    if (
      pendingAccessRequests.length === 0 ||
      createAttempt.status === 'processing' ||
      isInvalidRoleSelection
    )
      return true;
    if (
      fetchResourceRequestRolesAttempt.status === 'failed' &&
      hasUnsupporteKubeResourceKinds
    )
      return true;
    if (fetchResourceRequestRolesAttempt.status === 'processing') return true;
    if (longTerm) {
      return !dryRunResponse?.longTermResourceGrouping?.canProceed;
    }
    return false;
  }, [
    createAttempt,
    fetchResourceRequestRolesAttempt,
    hasUnsupporteKubeResourceKinds,
    isInvalidRoleSelection,
    longTerm,
    dryRunResponse?.longTermResourceGrouping,
    pendingAccessRequests,
  ]);

  const cancelBtnDisabled =
    createAttempt.status === 'processing' ||
    fetchResourceRequestRolesAttempt.status === 'processing';

  const [longTermDisabled, longTermDisabledReason] = useMemo(() => {
    // If long-term is already enabled, don't block toggling it off
    if (longTerm || dryRunResponse?.longTermResourceGrouping?.canProceed)
      return [false, undefined];
    if (!isResourceRequest)
      return [
        true,
        'Long-term access is only supported for resource-based requests.',
      ];
    // If canProceed is false on initial dryRun, show validation msg and disable until req is re-run.
    if (dryRunResponse?.longTermResourceGrouping?.canProceed === false) {
      return [
        true,
        'Long-term access is unavailable. ' +
          dryRunResponse.longTermResourceGrouping.validationMessage || '',
      ];
    }
    return [false, undefined];
  }, [isResourceRequest, longTerm, dryRunResponse?.longTermResourceGrouping]);

  const numPendingAccessRequests = pendingAccessRequests.filter(
    item => !isKubeClusterWithNamespaces(item, pendingAccessRequests)
  ).length;

  const DefaultHeader = () => {
    return (
      <Flex mb={3} alignItems="center">
        <ArrowBack
          size="large"
          mr={3}
          data-testid="close-checkout"
          onClick={onClose}
          style={{ cursor: 'pointer' }}
        />
        <Box>
          <H2>
            {numPendingAccessRequests}{' '}
            {pluralize(numPendingAccessRequests, 'Resource')} Selected
          </H2>
        </Box>
      </Flex>
    );
  };

  function customRow(item: T) {
    if (item.kind === 'kube_cluster') {
      return (
        <td colSpan={showClusterNameColumn ? 4 : 3}>
          <Flex>
            <Flex flexWrap="wrap">
              <Flex
                gap={2}
                justifyContent="space-between"
                width="100%"
                alignItems="center"
              >
                <Flex gap={5}>
                  {showClusterNameColumn && <Box>{item.clusterName}</Box>}
                  <Box>{getPrettyResourceKind(item.kind)}</Box>
                  <Box>{item.name}</Box>
                </Flex>
                <CrossIcon
                  clearAttempt={clearAttempt}
                  item={item}
                  toggleResource={toggleResource}
                  createAttempt={createAttempt}
                />
              </Flex>
              <KubeNamespaceSelector
                kubeClusterItem={item}
                savedResourceItems={pendingAccessRequests}
                fetchKubeNamespaces={fetchKubeNamespaces}
                updateNamespacesForKubeCluster={updateNamespacesForKubeCluster}
              />
            </Flex>
          </Flex>
        </td>
      );
    }
  }

  function getStyle(item: T) {
    if (
      !isResourceRequest ||
      !dryRunResponse?.longTermResourceGrouping ||
      !longTerm ||
      dryRunResponse.longTermResourceGrouping.canProceed
    ) {
      return;
    }
    const grouping =
      dryRunResponse.longTermResourceGrouping.optimalGrouping || [];

    if (!grouping.length || !grouping.some(i => i.name === item.name)) {
      return {
        background: theme.colors.interactive.tonal.danger[0],
        borderTopColor: theme.colors.interactive.tonal.danger[2],
      };
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          {!isRequestKubeResourceError &&
            createAttempt.status !== 'failed' &&
            fetchResourceRequestRolesAttempt.status === 'failed' && (
              <Alert kind="danger">
                {fetchResourceRequestRolesAttempt.statusText}
              </Alert>
            )}
          {hasUnsupporteKubeResourceKinds && (
            <Alert kind="danger">
              <HoverTooltip
                placement="left"
                tipContent={
                  fetchResourceRequestRolesAttempt.statusText.length > 248
                    ? fetchResourceRequestRolesAttempt.statusText
                    : null
                }
              >
                <ShortenedText mb={2}>
                  {fetchResourceRequestRolesAttempt.statusText}
                </ShortenedText>
              </HoverTooltip>
              <Text mb={2}>
                The listed allowed kinds are currently only supported through
                the{' '}
                <ExternalLink
                  target="_blank"
                  href="https://goteleport.com/docs/connect-your-client/tsh/#installing-tsh"
                >
                  tsh CLI tool
                </ExternalLink>
                . Use the{' '}
                <ExternalLink
                  target="_blank"
                  href="https://goteleport.com/docs/admin-guides/access-controls/access-requests/resource-requests/#search-for-kubernetes-resources"
                >
                  tsh request search
                </ExternalLink>{' '}
                that will help you construct the request.
              </Text>
              <Box width="325px">
                Example:
                <TextSelectCopyMulti
                  lines={[
                    {
                      text: `tsh request search --kind=ALLOWED_KIND --kube-cluster=CLUSTER_NAME --all-kube-namespaces`,
                    },
                  ]}
                />
              </Box>
            </Alert>
          )}
          {fetchStatus === 'loading' && (
            <Box mt={5} textAlign="center">
              <Indicator />
            </Box>
          )}

          {fetchStatus === 'loaded' && (
            <div>
              {createAttempt.status === 'success' ? (
                <>
                  <Box>
                    <Box as="header" mt={2} mb={7} textAlign="center">
                      <H2 mb={1}>Resources Requested Successfully</H2>
                      <Subtitle2 color="text.slightlyMuted">
                        You've successfully requested {numRequestedResources}{' '}
                        {pluralize(numRequestedResources, 'resource')}
                      </Subtitle2>
                    </Box>
                    <Flex justifyContent="center" mb={3}>
                      <Image src={shieldCheck} width="250px" height="179px" />
                    </Flex>
                  </Box>
                  <SuccessComponent onClose={onClose} reset={reset} />
                </>
              ) : (
                <>
                  {Header?.() || DefaultHeader()}
                  {createAttempt.status === 'failed' && (
                    <Alert kind="danger">{createAttempt.statusText}</Alert>
                  )}
                  {!longTermDisabled && (
                    <LongTermGroupingError
                      grouping={dryRunResponse?.longTermResourceGrouping}
                      toggleResources={toggleResources}
                      pendingAccessRequests={pendingAccessRequests}
                    />
                  )}
                  <StyledTable
                    data={pendingAccessRequests.filter(
                      d => d.kind !== 'namespace'
                    )}
                    row={{
                      customRow,
                      getStyle,
                    }}
                    columns={[
                      {
                        key: 'clusterName',
                        headerText: 'Cluster Name',
                        isNonRender: !showClusterNameColumn,
                      },
                      {
                        key: 'kind',
                        headerText: 'Type',
                        render: item => (
                          <Cell>{getPrettyResourceKind(item.kind)}</Cell>
                        ),
                      },
                      {
                        key: 'name',
                        headerText: 'Name',
                      },
                      {
                        altKey: 'delete-btn',
                        render: resource => (
                          <Cell align="right">
                            <CrossIcon
                              clearAttempt={clearAttempt}
                              item={resource}
                              toggleResource={toggleResource}
                              createAttempt={createAttempt}
                            />
                          </Cell>
                        ),
                      },
                    ]}
                    emptyText="No resources are selected"
                  />
                  {userGroupFetchAttempt?.status === 'processing' && (
                    <Flex mt={4} alignItems="center" justifyContent="center">
                      <Indicator size="small" />
                    </Flex>
                  )}
                  {userGroupFetchAttempt?.status === 'failed' && (
                    <Danger mt={4}>{userGroupFetchAttempt.statusText}</Danger>
                  )}
                  {userGroupFetchAttempt?.status === 'success' &&
                    appsGrantedByUserGroup.length > 0 && (
                      <AppsGrantedAccess apps={appsGrantedByUserGroup} />
                    )}
                  {isResourceRequest && (
                    <ResourceRequestRoles
                      roles={resourceRequestRoles}
                      selectedRoles={selectedResourceRequestRoles}
                      setSelectedRoles={setSelectedResourceRequestRoles}
                      fetchAttempt={fetchResourceRequestRolesAttempt}
                    />
                  )}
                  {/* Selecting reviewers not available for long-term requests */}
                  {(longTermDisabled || !longTerm) && (
                    <Box mt={4}>
                      <SelectReviewers
                        reviewers={
                          dryRunResponse?.reviewers.map(r => r.name) ?? []
                        }
                        selectedReviewers={selectedReviewers}
                        setSelectedReviewers={setSelectedReviewers}
                      />
                    </Box>
                  )}
                  <Flex flexDirection="column" gap={2} mt={4}>
                    <Text bold>Request type</Text>
                    <HoverTooltip
                      tipContent={longTermDisabledReason}
                      placement="left"
                    >
                      <Toggle
                        isToggled={longTerm}
                        onToggle={() => setLongTerm(v => !v)}
                        disabled={longTermDisabled}
                        css={
                          longTermDisabled && `cursor: not-allowed !important;`
                        }
                      >
                        <Flex flexDirection="column" ml={3}>
                          <Text>Long-term Access</Text>
                          <Text color="text.slightlyMuted" fontSize={1}>
                            You&#39;ll need to be added to an Access List
                          </Text>
                        </Flex>
                      </Toggle>
                    </HoverTooltip>
                  </Flex>
                  <Divider />
                  <Flex flexDirection="column" gap={1}>
                    {/* Start time / max duration are only valid for non-Long-Term requests */}
                    {dryRunResponse && !longTerm && (
                      <Box mb={1}>
                        <AssumeStartTime
                          start={startTime}
                          onStartChange={onStartTimeChange}
                          accessRequest={dryRunResponse}
                        />
                        <AccessDurationRequest
                          maxDuration={maxDuration}
                          onMaxDurationChange={onMaxDurationChange}
                          maxDurationOptions={maxDurationOptions}
                        />
                      </Box>
                    )}
                    <TextBox
                      reason={reason}
                      updateReason={updateReason}
                      requireReason={requireReason}
                      reasonPrompts={reasonPrompts}
                    />
                    {/* These options are only valid for non-Long-Term requests */}
                    {dryRunResponse && maxDuration && !longTerm && (
                      <AdditionalOptions
                        selectedMaxDurationTimestamp={maxDuration.value}
                        setPendingRequestTtl={setPendingRequestTtl}
                        pendingRequestTtl={pendingRequestTtl}
                        dryRunResponse={dryRunResponse}
                        pendingRequestTtlOptions={pendingRequestTtlOptions}
                      />
                    )}
                    <Divider />
                    <Flex
                      pb={4}
                      gap={2}
                      css={`
                        position: sticky;
                        bottom: 0;
                        background: ${({ theme }) =>
                          theme.colors.levels.sunken};
                        border-top: 1px solid
                          ${props => props.theme.colors.spotBackground[1]};
                      `}
                    >
                      <ButtonPrimary
                        width="100%"
                        size="large"
                        textTransform="none"
                        onClick={() => handleOnSubmit(validator)}
                        disabled={submitBtnDisabled}
                      >
                        Submit Request
                      </ButtonPrimary>
                      <ButtonSecondary
                        width="100%"
                        textTransform="none"
                        size="large"
                        onClick={() => {
                          reset();
                          onClose();
                        }}
                        disabled={cancelBtnDisabled}
                      >
                        Cancel
                      </ButtonSecondary>
                    </Flex>
                  </Flex>
                </>
              )}
            </div>
          )}
        </>
      )}
    </Validation>
  );
}

const LongTermGroupingError = <T extends PendingListItem = PendingListItem>({
  grouping,
  toggleResources,
  pendingAccessRequests,
}: {
  grouping: LongTermResourceGrouping;
  toggleResources?: RequestCheckoutProps<T>['toggleResources'];
  pendingAccessRequests: T[];
}) => {
  if (!grouping) return null;
  if (grouping.validationMessage?.length) {
    return (
      <StyledAlert
        kind="danger"
        primaryAction={getActionForLongTermGroupingError(
          grouping,
          pendingAccessRequests,
          toggleResources
        )}
      >
        {getMessageForLongTermGroupingError(grouping, pendingAccessRequests)}
      </StyledAlert>
    );
  }

  return null;
};

// StyledAlert is a modified version of Alert to match designs for LongTerm
// errors - more space is required so elements are stacked vertically instead
// of horizontally in a Flex.
const StyledAlert = styled(Alert)`
  > div {
    flex-wrap: wrap;
    gap: ${props => props.theme.space[2]}px;
  }
  > div > div:nth-child(1) {
    align-self: flex-start;

    margin-top: ${props => props.theme.space[1]}px;
    margin-right: 0;
    flex-shrink: 0;
    flex-grow: 0;
  }
  > div > div:nth-child(2) {
    flex-shrink: 0;
  }
  > div > div:nth-child(3) {
    flex-basis: 100%;
    margin-left: 0;
  }
`;

// findIncompatibleLongTermResources iterates through the
// pendingRequests and returns a list of resources
// that are incompatible with the optimalGrouping.
const findIncompatibleLongTermResources = <
  T extends PendingListItem = PendingListItem,
>(
  grouping: LongTermResourceGrouping,
  pendingRequests: T[]
) => {
  return pendingRequests.filter(
    item =>
      item.kind !== 'namespace' &&
      !grouping.optimalGrouping.some(i => i.name === item.name)
  );
};

// getActionForLongTermGroupingError returns an action object
// for the primaryAction of the StyledAlert, used to remove
// incompatible resources if any are present.
const getActionForLongTermGroupingError = <
  T extends PendingListItem = PendingListItem,
>(
  grouping: LongTermResourceGrouping,
  pendingRequests: T[],
  toggleResources?: RequestCheckoutProps<T>['toggleResources']
) => {
  if (
    grouping.optimalGrouping.length <
    pendingRequests.filter(i => i.kind !== 'namespace').length
  ) {
    if (!grouping.optimalGrouping.length || !toggleResources) {
      return undefined;
    }

    const incompatibleResources = findIncompatibleLongTermResources(
      grouping,
      pendingRequests
    );
    if (!incompatibleResources.length) {
      return undefined;
    }

    return {
      content: 'Remove Incompatible Resources',
      onClick: () =>
        toggleResources(
          incompatibleResources.map(i => ({
            resourceName: i.name,
            resourceId: i.id,
            kind: i.kind,
          }))
        ),
    };
  }
};

// getMessageForLongTermGroupingError returns a parsed/used-readable
// message for the long-term grouping error.
const getMessageForLongTermGroupingError = <
  T extends PendingListItem = PendingListItem,
>(
  grouping: LongTermResourceGrouping,
  pendingRequests: T[]
) => {
  const message =
    grouping.validationMessage || 'Long-term access is unavailable.';

  let subText = '';

  if (Object.keys(grouping.groupedByAccessList).length > 1) {
    const incompatibleResources = findIncompatibleLongTermResources(
      grouping,
      pendingRequests
    );
    subText = `Remove ${incompatibleResources
      .map(i => i.name)
      .join(
        ', '
      )} and request them separately, or switch to a short-term request.`;
  }

  if (!grouping.optimalGrouping.length) {
    subText =
      'No selected resources are available for long-term access. Please switch to a short-term request to continue.';
  }

  return (
    <Flex flexDirection="column" gap={1}>
      <Text>{message}</Text>
      <Text typography="body2" bold={false}>
        {subText}
      </Text>
    </Flex>
  );
};

const Divider = styled.div`
  width: 100%;
  height: 1px;
  pointer-events: none;
  background-color: ${props => props.theme.colors.spotBackground[1]};
  margin-top: ${props => props.theme.space[4]}px;
  margin-bottom: ${props => props.theme.space[4]}px;
`;

function AppsGrantedAccess({ apps }: { apps: string[] }) {
  const [expanded, setExpanded] = useState(true);
  const ArrowIcon = expanded ? ChevronDown : ChevronRight;

  // if its a single app, just show the app they are getting access to
  if (apps.length === 1) {
    return (
      <Box mt={4} width="100%">
        <Text mb={0}>
          Grants access to the{' '}
          <Text style={{ display: 'inline' }} color="brand">
            {apps[0]}
          </Text>{' '}
          app
        </Text>
      </Box>
    );
  }

  return (
    <Box mt={7} width="100%">
      <Box style={{ cursor: 'pointer' }}>
        <Flex
          justifyContent="space-between"
          width="100%"
          borderBottom={1}
          onClick={() => setExpanded(!expanded)}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[1]};
          `}
        >
          <Flex flexDirection="column" width="100%">
            <LabelInput mb={0} style={{ cursor: 'pointer' }}>
              {`Grants access to ${apps.length} apps`}
            </LabelInput>
          </Flex>
          {apps.length > 0 && <ArrowIcon size="medium" />}
        </Flex>
      </Box>
      {expanded && (
        <Box mt={2}>
          {apps.map(app => {
            return <Text>{app}</Text>;
          })}
        </Box>
      )}
    </Box>
  );
}

function ResourceRequestRoles({
  roles,
  selectedRoles,
  setSelectedRoles,
  fetchAttempt,
}: {
  roles: string[];
  selectedRoles: string[];
  setSelectedRoles: (roles: string[]) => void;
  fetchAttempt: Attempt;
}) {
  const [expanded, setExpanded] = useState(true);
  const ArrowIcon = expanded ? ChevronDown : ChevronRight;

  function onInputChange(
    roleName: string,
    e: React.ChangeEvent<HTMLInputElement>
  ) {
    if (e.target.checked) {
      return setSelectedRoles([...selectedRoles, roleName]);
    }
    setSelectedRoles(selectedRoles.filter(role => role !== roleName));
  }
  // only show the role selector if there is more than one role that can be selected
  if (roles.length < 2) {
    return;
  }

  return (
    <Box mt={7} width="100%">
      <Box style={{ cursor: 'pointer' }}>
        <Flex
          justifyContent="space-between"
          width="100%"
          borderBottom={1}
          onClick={() => setExpanded(!expanded)}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[1]};
          `}
        >
          <Flex alignItems="center" gap={2}>
            <Flex flexDirection="column" width="100%">
              <LabelInput mb={0} style={{ cursor: 'pointer' }}>
                Roles
              </LabelInput>
              <Text typography="body4" mb={2}>
                {selectedRoles.length} role
                {selectedRoles.length !== 1 ? 's' : ''} selected
              </Text>
            </Flex>
            {selectedRoles.length ? (
              <ButtonBorder
                onClick={() => setSelectedRoles([])}
                size="small"
                width="50px"
              >
                Clear
              </ButtonBorder>
            ) : null}
          </Flex>
          {fetchAttempt.status === 'processing' ? (
            <Flex
              mt={3}
              mr={1}
              height="100%"
              alignItems="center"
              justifyContent="center"
            >
              <Indicator size="medium" />
            </Flex>
          ) : (
            <Flex
              mt={2}
              height="100%"
              alignItems="center"
              justifyContent="center"
            >
              <ButtonIcon>
                <ArrowIcon size="medium" />
              </ButtonIcon>
            </Flex>
          )}
        </Flex>
      </Box>
      {fetchAttempt.status === 'success' && expanded && (
        <Box mt={2}>
          {roles.map((roleName, index) => {
            const checked = selectedRoles.includes(roleName);
            return (
              <RoleRowContainer checked={checked} key={index}>
                <StyledFieldCheckbox
                  key={index}
                  name={roleName}
                  onChange={e => {
                    onInputChange(roleName, e);
                  }}
                  checked={checked}
                  label={roleName}
                  size="small"
                />
              </RoleRowContainer>
            );
          })}
          {selectedRoles.length < roles.length && (
            <Flex
              alignItems="center"
              justifyContent="space-between"
              mt={3}
              py={2}
              px={3}
              borderRadius={3}
              css={`
                width: 100%;
                background: ${({ theme }) => theme.colors.levels.surface};
              `}
            >
              <Warning mr={3} size="medium" color="warning.main" />
              <P3>
                Modifying this role set may disable access to some of the above
                resources. Use with caution.
              </P3>
            </Flex>
          )}
        </Box>
      )}
    </Box>
  );
}

const RoleRowContainer = styled.div<{ checked?: boolean }>`
  transition: all 150ms;
  position: relative;

  // TODO(bl-nero): That's the third place where we're copying these
  // definitions. We need to make them reusable.
  &:hover {
    background-color: ${props => props.theme.colors.levels.surface};

    // We use a pseudo element for the shadow with position: absolute in order to prevent
    // the shadow from increasing the size of the layout and causing scrollbar flicker.
    &:after {
      box-shadow: ${props => props.theme.boxShadow[3]};
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      z-index: -1;
      width: 100%;
      height: 100%;
    }
  }
`;

const StyledFieldCheckbox = styled(FieldCheckbox)`
  margin: 0;
  padding: ${p => p.theme.space[2]}px;
  background-color: ${props =>
    props.checked
      ? props.theme.colors.interactive.tonal.primary[2]
      : 'transparent'};
  border-bottom: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.interactive.tonal.neutral[0]};

  & > label {
    display: block; // make it full-width
  }
`;

function TextBox({
  reason,
  updateReason,
  requireReason,
  reasonPrompts,
}: {
  reason: string;
  updateReason(reason: string): void;
  requireReason: boolean;
  reasonPrompts?: string[];
}) {
  const { valid, message } = useRule(requireText(reason, requireReason));
  const hasError = !valid;
  const labelText = hasError ? message : 'Request Reason';

  const placeholderText =
    reasonPrompts?.filter(s => s.length > 0).join('\n') ||
    'Describe your request...';

  return (
    <LabelInput hasError={hasError}>
      <Text mb={1}>{labelText}</Text>
      <Box
        as="textarea"
        height="80px"
        width="100%"
        borderRadius={2}
        p={2}
        color="text.main"
        border={hasError ? '2px solid' : '1px solid'}
        borderColor={hasError ? 'error.main' : 'text.muted'}
        placeholder={placeholderText.replaceAll(/\\n/g, '\n')}
        value={reason}
        onChange={e => updateReason(e.target.value)}
        css={`
          outline: none;
          background: transparent;
          font-size: ${props => props.theme.fontSizes[2]}px;

          &::placeholder {
            color: ${({ theme }) => theme.colors.text.muted};
          }

          &:hover,
          &:focus,
          &:active {
            border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
          }
        `}
      />
    </LabelInput>
  );
}

function getPrettyResourceKind(kind: RequestableResourceKind): string {
  switch (kind) {
    case 'role':
      return 'Role';
    case 'app':
      return 'Application';
    case 'node':
      return 'Server';
    case 'resource':
      return 'Resource';
    case 'db':
      return 'Database';
    case 'kube_cluster':
      return 'Kubernetes';
    case 'user_group':
      return 'User Group';
    case 'windows_desktop':
      return 'Desktop';
    case 'saml_idp_service_provider':
      return 'SAML Application';
    case 'namespace':
      return 'Namespace';
    case 'aws_ic_account_assignment':
      return 'AWS IAM Identity Center Account Assignment';
    case 'git_server':
      return 'Git';
    default:
      kind satisfies never;
      return kind;
  }
}

const requireText = (value: string, requireReason: boolean) => () => {
  if (requireReason && (!value || value.trim().length === 0)) {
    return {
      valid: false,
      message: 'Reason Required',
    };
  }
  return { valid: true };
};

const SidePanel = styled(Box)`
  position: absolute;
  // This z-index must be a higher value than the top bar z-index defined for
  // Teleport web UI navigation found in teleport/src/Navigation/zIndexMap.ts.
  // It prevents this SidePanel from rendering underneath the navigation bits.
  z-index: 100;
  top: 0px;
  right: 0px;
  background: ${({ theme }) => theme.colors.levels.sunken};
  min-height: 100%;
  width: 500px;
  padding: 20px;

  &.entering {
    right: -500px;
  }

  &.entered {
    right: 0px;
    transition: right 300ms ease-out;
  }

  &.exiting {
    right: -500px;
    transition: right 300ms ease-out;
  }

  &.exited {
    right: -500px;
  }
`;

const Dimmer = styled(Box)`
  background: #000;
  opacity: 0.5;
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 10;
`;

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
  }

  & > thead > tr > th {
    background: ${props => props.theme.colors.spotBackground[1]};
  }

  border-radius: 8px;
  box-shadow: ${props => props.theme.boxShadow[0]};
  overflow: hidden;
` as typeof Table;

const ShortenedText = styled(Text)`
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 6;
`;

export type RequestCheckoutWithSliderProps<
  T extends PendingListItem = PendingListItem,
> = {
  transitionState: TransitionStatus;
} & RequestCheckoutProps<T>;

export interface PendingListItem {
  kind: RequestableResourceKind;
  /** Name of the resource, for presentation purposes only. */
  name: string;
  /** Identifier of the resource. Should be sent in requests. */
  id: string;
  clusterName?: string;
  /**
   * This field must be defined if a user is requesting subresources.
   *
   * Example:
   * "kube_cluster" resource can have subresources such as "namespace".
   * Example PendingListItem values if user is requesting a kubes namespace:
   *   - kind: const "namespace"
   *   - id: name of the kube_cluster
   *   - subResourceName: name of the kube_cluster's namespace
   *   - clusterName: name of teleport cluster where kube_cluster is located
   *   - name: same as subResourceName as this is what we want to display to user
   * */
  subResourceName?: string;
}

export type PendingKubeResourceItem = Omit<PendingListItem, 'kind'> & {
  kind: Extract<RequestableResourceKind, 'namespace'>;
};

export type RequestCheckoutProps<T extends PendingListItem = PendingListItem> =
  {
    onClose(): void;
    toggleResource: (resource: T) => void;
    toggleResources: (
      resources: {
        kind: T['kind'];
        resourceId: T['id'];
        resourceName: T['name'];
      }[],
      action?: 'add' | 'remove'
    ) => void;
    appsGrantedByUserGroup?: string[];
    userGroupFetchAttempt?: Attempt;
    reset: () => void;
    SuccessComponent?: (params: SuccessComponentParams) => JSX.Element;
    isResourceRequest: boolean;
    requireReason: boolean;
    reasonPrompts: string[];
    selectedReviewers: ReviewerOption[];
    pendingAccessRequests: T[];
    showClusterNameColumn?: boolean;
    createRequest: (req: CreateRequest) => void;
    fetchStatus: 'loading' | 'loaded';
    fetchResourceRequestRolesAttempt: Attempt;
    pendingRequestTtl: Option<number>;
    setPendingRequestTtl: (value: Option<number>) => void;
    pendingRequestTtlOptions: Option<number>[];
    resourceRequestRoles: string[];
    maxDuration: Option<number>;
    onMaxDurationChange: (value: Option<number>) => void;
    maxDurationOptions: Option<number>[];
    setSelectedReviewers: (value: ReviewerOption[]) => void;
    clearAttempt: () => void;
    createAttempt: Attempt;
    setSelectedResourceRequestRoles: (value: string[]) => void;
    numRequestedResources: number;
    selectedResourceRequestRoles: string[];
    dryRunResponse: AccessRequest;
    Header?: () => JSX.Element;
    startTime: Date;
    longTerm: boolean;
    setLongTerm: React.Dispatch<React.SetStateAction<boolean>>;
    onStartTimeChange(t?: Date): void;
    fetchKubeNamespaces(search: string, kubeCluster: T): Promise<string[]>;
    updateNamespacesForKubeCluster(
      kubeResources: PendingKubeResourceItem[],
      kubeCluster: T
    ): void;
  };

type SuccessComponentParams = {
  reset: () => void;
  onClose: () => void;
};
