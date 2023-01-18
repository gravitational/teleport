import React, { useState, useRef } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import {
  Box,
  Flex,
  ButtonText,
  ButtonPrimary,
  Image,
  Text,
  LabelInput,
  Alert,
  Indicator,
} from 'design';
import { ArrowBack, Trash, ArrowDown, ArrowRight, Warning } from 'design/Icon';
import Table, { Cell } from 'design/DataTable';
import { CheckboxInput, CheckboxWrapper } from 'design/Checkbox';
import Validation, { useRule, Validator } from 'shared/components/Validation';
import { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { pluralize } from 'teleport/lib/util';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';

import { State as NewRequestState } from '../useNewRequest';

import shieldCheck from './shield-check.png';
import { SelectReviewers } from './SelectReviewers';
import { State, useRequestCheckout } from './useRequestCheckout';

import type { TransitionStatus } from 'react-transition-group';

type CreateOption = Option & {
  isDisabled?: boolean;
  isSelected?: boolean;
};

export function SuccessActionComponent({ cfg, reset, onClose }) {
  return (
    <Box textAlign="center">
      <ButtonPrimary
        as={Link}
        mt={5}
        mb={3}
        width="100%"
        size="large"
        to={cfg.getAccessRequestRoute()}
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

export default function Container(props: Props) {
  const { selectedResource, addedResources, reset } = props;
  const ctx = useTeleportE();
  const state = useRequestCheckout({
    ctx,
    selectedResource,
    addedResources,
    reset,
  });

  return (
    <RequestCheckout
      {...state}
      {...props}
      SuccessComponent={SuccessActionComponent}
    />
  );
}

export function RequestCheckout({
  toggleResource,
  onClose,
  transitionState,
  reset,
  data,
  createAttempt,
  fetchResourceRequestRolesAttempt,
  resourceRequestRoles,
  createRequest,
  clearAttempt,
  reviewers,
  SuccessComponent,
  requireReason,
  numRequestedResources,
  isResourceRequest,
  selectedResourceRequestRoles,
  setSelectedResourceRequestRoles,
}: RequestCheckoutProps) {
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

  const [selectedReviewers, setSelectedReviewers] = useState<CreateOption[]>(
    []
  );

  function updateReason(reason: string) {
    setReason(reason);
  }

  function handleOnSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    createRequest(
      reason,
      selectedReviewers.map(r => r.value)
    );
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
        {createAttempt.status === 'success' ? (
          <Box>
            <Box mt={2} mb={7} textAlign="center">
              <Text typography="h4" color="light" bold>
                Resources Requested Successfully
              </Text>
              <Text typography="subtitle1" color="text.secondary">
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
              fontSize={25}
              mr={3}
              onClick={onClose}
              style={{ cursor: 'pointer' }}
            />
            <Box>
              <Text typography="h4" color="light" bold>
                {data.length} {pluralize(data.length, 'Resource')} Selected
              </Text>
            </Box>
          </Flex>
        )}
        {createAttempt.status === 'success' ? (
          <SuccessComponent cfg={cfg} onClose={onClose} reset={reset} />
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
                        fontSize={13}
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
                          background-color: #2e3860;
                          border-radius: 2px;
                          :hover {
                            background-color: #414b70;
                          }
                        `}
                      />
                    </Cell>
                  ),
                },
              ]}
              emptyText="No resources are selected"
            />
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
                <>
                  <TextBox
                    reason={reason}
                    updateReason={updateReason}
                    requireReason={requireReason}
                  />
                  <Box
                    py={4}
                    css={`
                      position: sticky;
                      bottom: 0;
                      background: ${({ theme }) => theme.colors.primary.dark};
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
                </>
              )}
            </Validation>
          </>
        )}
      </SidePanel>
    </div>
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
  const [expanded, setExpanded] = useState(false);
  const ArrowIcon = expanded ? ArrowDown : ArrowRight;

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
          borderColor="primary.main"
          onClick={() => setExpanded(!expanded)}
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
            <Box height="100%">
              <Indicator fontSize="16px" />
            </Box>
          ) : (
            <Flex
              mt={3}
              mr={1}
              height="100%"
              alignItems="center"
              justifyContent="center"
            >
              <ArrowIcon fontSize="16px" />
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
                  background: ${({ theme }) => theme.colors.primary.light};
                  &:hover {
                    border-color: ${({ theme }) =>
                      theme.colors.primary.lighter};
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
                background: ${({ theme }) => theme.colors.primary.light};
              `}
            >
              <Warning mr={3} fontSize="16px" color="warning" />
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
    <Box mt={7}>
      <LabelInput hasError={hasError}>{labelText}</LabelInput>
      <Box
        as="textarea"
        height="80px"
        width="100%"
        borderRadius={2}
        p={2}
        color={'text.primary'}
        border={hasError ? '2px solid' : '1px solid'}
        borderColor={hasError ? 'error.dark' : 'primary.light'}
        style={{ outline: 'none' }}
        placeholder={placeholder}
        value={reason}
        onChange={e => updateReason(e.target.value)}
        css={`
          background: ${({ theme }) => theme.colors.primary.main};
          ::placeholder {
            color: ${({ theme }) => theme.colors.text.secondary};
          }
        `}
      />
    </Box>
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
  background: ${({ theme }) => theme.colors.primary.dark};
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
` as typeof Table;

type Props = {
  onClose(): void;
  selectedResource: NewRequestState['selectedResource'];
  toggleResource: NewRequestState['addOrRemoveResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
  SuccessComponent?: (params: SuccessComponentParams) => JSX.Element;
  transitionState: TransitionStatus;
  isResourceRequest: boolean;
};

type SuccessComponentParams = {
  cfg: typeof cfg;
  reset: () => void;
  onClose: () => void;
};

export type RequestCheckoutProps = Omit<
  Props,
  'addedResources' | 'selectedResource'
> &
  State;
