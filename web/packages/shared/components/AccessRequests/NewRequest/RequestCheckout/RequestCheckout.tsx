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

import React, { useRef, useState } from 'react';
import styled from 'styled-components';
import {
  Alert,
  Box,
  ButtonIcon,
  ButtonPrimary,
  Flex,
  Image,
  Indicator,
  LabelInput,
  Text,
} from 'design';
import {
  ArrowBack,
  ChevronDown,
  ChevronRight,
  Trash,
  Warning,
} from 'design/Icon';
import Table, { Cell } from 'design/DataTable';
import { CheckboxInput, CheckboxWrapper } from 'design/Checkbox';

import { Danger } from 'design/Alert';

import Validation, { useRule, Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { pluralize } from 'shared/utils/text';
import { Option } from 'shared/components/Select';

import { CreateRequest } from '../../Shared/types';
import { AssumeStartTime } from '../../AssumeStartTime/AssumeStartTime';
import { AccessDurationRequest } from '../../AccessDuration';

import { ReviewerOption } from './types';

import shieldCheck from './shield-check.png';
import { SelectReviewers } from './SelectReviewers';
import { AdditionalOptions } from './AdditionalOptions';

import type { TransitionStatus } from 'react-transition-group';

import type { AccessRequest } from 'shared/services/accessRequests';
import type { ResourceKind } from '../resource';

export function RequestCheckout({
  toggleResource,
  onClose,
  transitionState,
  reset,
  data,
  createAttempt,
  appsGrantedByUserGroup = [],
  userGroupFetchAttempt,
  fetchResourceRequestRolesAttempt,
  resourceRequestRoles,
  createRequest,
  clearAttempt,
  reviewers,
  selectedReviewers,
  setSelectedReviewers,
  SuccessComponent,
  requireReason,
  numRequestedResources,
  isResourceRequest,
  selectedResourceRequestRoles,
  setSelectedResourceRequestRoles,
  fetchStatus,
  maxDuration,
  setMaxDuration,
  requestTTL,
  setRequestTTL,
  dryRunResponse,
}: RequestCheckoutProps) {
  // Specifies the start date/time a requestor requested for.
  const [start, setStart] = useState<Date>();
  const [reason, setReason] = useState('');
  const ref = useRef<HTMLDivElement>();

  const isInvalidRoleSelection =
    resourceRequestRoles.length > 0 &&
    isResourceRequest &&
    selectedResourceRequestRoles.length < 1;
  const submitBtnDisabled =
    data.length === 0 ||
    createAttempt.status === 'processing' ||
    isInvalidRoleSelection ||
    fetchResourceRequestRolesAttempt.status === 'failed' ||
    fetchResourceRequestRolesAttempt.status === 'processing';

  function updateReason(reason: string) {
    setReason(reason);
  }

  function handleOnSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    createRequest({
      reason,
      suggestedReviewers: selectedReviewers.map(r => r.value),
      maxDuration: maxDuration ? new Date(maxDuration.value) : null,
      requestTTL: requestTTL ? new Date(requestTTL.value) : null,
      start: start,
    });
  }

  // Listeners are attached to enable overflow on the parent container after
  // transitioning ends (entered) or starts (exits). Enables vertical scrolling
  // when content gets too big.
  //
  // Overflow is initially hidden to prevent
  // brief flashing of horizontal scroll bar resulting from positioning
  // the container off screen to the right for the slide affect.
  React.useEffect(() => {
    function applyOverflowAutoStyle(e: TransitionEvent) {
      if (e.propertyName === 'right') {
        ref.current.style.overflow = `auto`;
        // There will only ever be one 'end right' transition invoked event, so we remove it
        // afterwards, and listen for the 'start right' transition which is only invoked
        // when user exits this component.
        window.removeEventListener('transitionend', applyOverflowAutoStyle);
        window.addEventListener('transitionstart', applyOverflowHiddenStyle);
      }
    }

    function applyOverflowHiddenStyle(e: TransitionEvent) {
      if (e.propertyName === 'right') {
        ref.current.style.overflow = `hidden`;
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
      ref={ref}
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
      <SidePanel state={transitionState} className={transitionState}>
        {fetchResourceRequestRolesAttempt.status === 'failed' && (
          <Alert
            kind="danger"
            children={fetchResourceRequestRolesAttempt.statusText}
          />
        )}
        {fetchStatus === 'loading' && (
          <Box mt={5} textAlign="center">
            <Indicator />
          </Box>
        )}

        {fetchStatus === 'loaded' && (
          <div>
            {createAttempt.status === 'success' ? (
              <Box>
                <Box mt={2} mb={7} textAlign="center">
                  <Text typography="h4" color="text.main" bold>
                    Resources Requested Successfully
                  </Text>
                  <Text typography="subtitle1" color="text.slightlyMuted">
                    You've successfully requested {numRequestedResources}{' '}
                    {pluralize(numRequestedResources, 'resource')}
                  </Text>
                </Box>
                <Flex justifyContent="center" mb={3}>
                  <Image src={shieldCheck} width="250px" height="179px" />
                </Flex>
              </Box>
            ) : (
              <Flex mb={3} alignItems="center">
                <ArrowBack
                  size="large"
                  mr={3}
                  onClick={onClose}
                  style={{ cursor: 'pointer' }}
                />
                <Box>
                  <Text typography="h4" color="text.main" bold>
                    {data.length} {pluralize(data.length, 'Resource')} Selected
                  </Text>
                </Box>
              </Flex>
            )}
            {createAttempt.status === 'success' ? (
              <SuccessComponent onClose={onClose} reset={reset} />
            ) : (
              <>
                {createAttempt.status === 'failed' && (
                  <Alert kind="danger" children={createAttempt.statusText} />
                )}
                <StyledTable
                  data={data}
                  columns={[
                    {
                      key: 'kind',
                      headerText: 'Resource Kind',
                    },
                    {
                      key: 'name',
                      headerText: 'Resource Name',
                    },
                    {
                      altKey: 'delete-btn',
                      render: resource => (
                        <Cell align="right">
                          <Trash
                            size="small"
                            borderRadius={2}
                            p={2}
                            onClick={() => {
                              clearAttempt();
                              toggleResource(
                                resource.kind,
                                resource.id,
                                resource.name
                              );
                            }}
                            disabled={createAttempt.status === 'processing'}
                            css={`
                              cursor: pointer;

                              background-color: ${({ theme }) =>
                                theme.colors.buttons.trashButton.default};
                              border-radius: 2px;

                              :hover {
                                background-color: ${({ theme }) =>
                                  theme.colors.buttons.trashButton.hover};
                              }
                            `}
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
                <Box mt={6} mb={1}>
                  <SelectReviewers
                    reviewers={reviewers}
                    selectedReviewers={selectedReviewers}
                    setSelectedReviewers={setSelectedReviewers}
                  />
                </Box>
                <Validation>
                  {({ validator }) => (
                    <Flex mt={6} flexDirection="column" gap={1}>
                      {dryRunResponse && (
                        <Box mb={1}>
                          <AssumeStartTime
                            start={start}
                            onStartChange={setStart}
                            accessRequest={dryRunResponse}
                          />
                          <AccessDurationRequest
                            assumeStartTime={start}
                            maxDuration={maxDuration}
                            setMaxDuration={setMaxDuration}
                            accessRequest={dryRunResponse}
                          />
                        </Box>
                      )}
                      <TextBox
                        reason={reason}
                        updateReason={updateReason}
                        requireReason={requireReason}
                      />
                      {dryRunResponse && maxDuration && (
                        <AdditionalOptions
                          selectedMaxDurationTimestamp={maxDuration?.value}
                          maxDuration={maxDuration}
                          setRequestTTL={setRequestTTL}
                          requestTTL={requestTTL}
                          dryRunResponse={dryRunResponse}
                        />
                      )}
                      <Box
                        py={4}
                        css={`
                          position: sticky;
                          bottom: 0;
                          background: ${({ theme }) =>
                            theme.colors.levels.sunken};
                        `}
                      >
                        <ButtonPrimary
                          width="100%"
                          size="large"
                          onClick={() => handleOnSubmit(validator)}
                          disabled={submitBtnDisabled}
                        >
                          Submit Request
                        </ButtonPrimary>
                      </Box>
                    </Flex>
                  )}
                </Validation>
              </>
            )}
          </div>
        )}
      </SidePanel>
    </div>
  );
}

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
              Roles
            </LabelInput>
            <Text typography="subtitle2" mb={2}>
              {selectedRoles.length} role{selectedRoles.length !== 1 ? 's' : ''}{' '}
              selected
            </Text>
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
            const id = `${roleName}${index}`;
            return (
              <CheckboxWrapper
                key={index}
                css={`
                  width: 100%;
                  cursor: pointer;
                  background: ${({ theme }) => theme.colors.levels.surface};

                  &:hover {
                    border-color: ${({ theme }) =>
                      theme.colors.levels.elevated};
                  }
                `}
                as="label"
                htmlFor={id}
              >
                <CheckboxInput
                  type="checkbox"
                  name={roleName}
                  id={id}
                  onChange={e => {
                    onInputChange(roleName, e);
                  }}
                  checked={selectedRoles.includes(roleName)}
                />
                {roleName}
              </CheckboxWrapper>
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
              <Text typography="subtitle2">
                Modifying this role set may disable access to some of the above
                resources. Use with caution.
              </Text>
            </Flex>
          )}
        </Box>
      )}
    </Box>
  );
}

function TextBox({
  reason,
  updateReason,
  requireReason,
}: {
  reason: string;
  updateReason(reason: string): void;
  requireReason: boolean;
}) {
  const { valid, message } = useRule(requireText(reason, requireReason));
  const hasError = !valid;
  const labelText = hasError ? message : 'Request Reason';

  const optionalText = requireReason ? '' : ' (optional)';
  const placeholder = `Describe your request...${optionalText}`;

  return (
    <LabelInput hasError={hasError}>
      {labelText}
      <Box
        as="textarea"
        height="80px"
        width="100%"
        borderRadius={2}
        p={2}
        color="text.main"
        border={hasError ? '2px solid' : '1px solid'}
        borderColor={hasError ? 'error.main' : 'text.muted'}
        placeholder={placeholder}
        value={reason}
        onChange={e => updateReason(e.target.value)}
        css={`
          outline: none;
          background: transparent;

          ::placeholder {
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
  z-index: 11;
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

export type RequestCheckoutProps = {
  onClose(): void;
  toggleResource: (
    kind: ResourceKind,
    resourceId: string,
    resourceName?: string
  ) => void;
  appsGrantedByUserGroup?: string[];
  userGroupFetchAttempt?: Attempt;
  reset: () => void;
  SuccessComponent?: (params: SuccessComponentParams) => JSX.Element;
  transitionState: TransitionStatus;
  isResourceRequest: boolean;
  requireReason: boolean;
  selectedReviewers: ReviewerOption[];
  data: {
    kind: ResourceKind;
    /** Name of the resource, for presentation purposes only. */
    name: string;
    /** Identifier of the resource. Should be sent in requests. */
    id: string;
  }[];
  setRequestTTL: (value: Option<number>) => void;
  createRequest: (req: CreateRequest) => void;
  fetchStatus: 'loading' | 'loaded';
  fetchResourceRequestRolesAttempt: Attempt;
  requestTTL: Option<number>;
  resourceRequestRoles: string[];
  reviewers: string[];
  setSelectedReviewers: (value: ReviewerOption[]) => void;
  setMaxDuration: (value: Option<number>) => void;
  clearAttempt: () => void;
  createAttempt: Attempt;
  setSelectedResourceRequestRoles: (value: string[]) => void;
  numRequestedResources: number;
  selectedResourceRequestRoles: string[];
  dryRunResponse: AccessRequest;
  maxDuration: Option<number>;
};

type SuccessComponentParams = {
  reset: () => void;
  onClose: () => void;
};
