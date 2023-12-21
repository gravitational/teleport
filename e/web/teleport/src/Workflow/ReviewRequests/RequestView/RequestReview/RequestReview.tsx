import React, { PropsWithChildren, useState } from 'react';
import styled from 'styled-components';
import {
  ButtonPrimary,
  Text,
  Box,
  LabelInput,
  Alert,
  Flex,
  Label,
  Popover,
} from 'design';
import { Warning } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';
import Validation, { Validator } from 'shared/components/Validation';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { requiredField } from 'shared/components/Validation/rules';

import { AccessRequest, RequestState } from 'e-teleport/services/workflow';
import { makeTraitLabel } from 'e-teleport/AccessListManagement/Traits';
import { AccessList } from 'e-teleport/services/accessmanagement';

import { State as RequestViewState } from '../useRequestView';
import { LongTermAccess } from '../types';

type ReviewStateOption = Option<RequestState, React.ReactElement> & {
  disabled?: boolean;
};

type SuggestedAcessListOption = Option<AccessList, React.ReactElement>;

export default function RequestReview({
  attempt,
  submitReview,
  user,
  longTermAccess,
  shortTermDuration,
  request,
}: Props) {
  const [reviewStateOptions] = useState<ReviewStateOption[]>(() =>
    makeReviewStateOptions(longTermAccess, shortTermDuration, request)
  );

  const [suggestedAccessListOptions] = useState<SuggestedAcessListOption[]>(
    () => makeSuggestedAccessListOptions(longTermAccess)
  );

  const [state, setState] = useState<RequestState>(reviewStateOptions[0].value);
  const [reason, setReason] = useState('');

  const [selectedAccessList, setSelectedAccessList] =
    useState<SuggestedAcessListOption>();

  function onSubmitReview(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    submitReview({
      state,
      reason,
      promotedToAccessList: selectedAccessList?.value,
    });
  }

  function onRequestStateChange(state: RequestState, validator: Validator) {
    validator.reset();
    if (state !== 'PROMOTED' && selectedAccessList) {
      setSelectedAccessList(undefined);
    }
    setState(state);
  }

  // After successful submit, don't render.
  if (attempt.status === 'success') {
    return null;
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box
          border="1px solid"
          borderColor="levels.sunken"
          mt={7}
          style={{ position: 'relative' }}
        >
          <Box bg="levels.sunken" py={1} px={3} alignItems="center">
            <Text typography="h6" mr={3}>
              {user} - add a review
            </Text>
          </Box>
          <Box p={3} bg="levels.elevated">
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box mb={3}>
              <RadioGroup
                name="requestState"
                options={reviewStateOptions}
                value={state}
                gap="8px"
                onChange={o =>
                  onRequestStateChange(o as RequestState, validator)
                }
              />
              {state === 'PROMOTED' && (
                <Box ml={4} mt={3} css={{ position: 'relative' }}>
                  <HorizontalLine />
                  <FieldSelect
                    ml={1}
                    width="600px"
                    label={`Select a suggested Access List to add ${request.user} as a member to`}
                    rule={requiredField('Required')}
                    placeholder={`Select a suggested Access List to add ${request.user} as a member to`}
                    value={
                      selectedAccessList
                        ? {
                            value: selectedAccessList,
                            label: selectedAccessList.value.title,
                          }
                        : undefined
                    }
                    onChange={(o: SuggestedAcessListOption) =>
                      setSelectedAccessList(o)
                    }
                    options={suggestedAccessListOptions}
                  />
                </Box>
              )}
            </Box>
            <Box mb={4}>
              <LabelInput mb={1}>Message</LabelInput>
              <Box
                width="100%"
                maxWidth="500px"
                height="150px"
                as="textarea"
                p={2}
                borderRadius={2}
                placeholder="Optional message..."
                color="text.main"
                border="1px solid"
                borderColor="text.muted"
                value={reason}
                onChange={e => setReason(e.target.value)}
                autoFocus
                css={`
                  outline: none;
                  background: transparent;
                  ::placeholder {
                    color: ${({ theme }) => theme.colors.text.muted};
                  }
                  &:hover,
                  &:focus,
                  &:active {
                    border: 1px solid
                      ${props => props.theme.colors.text.slightlyMuted};
                  }
                `}
              />
            </Box>
            <ButtonPrimary
              disabled={attempt.status === 'processing'}
              onClick={() => onSubmitReview(validator)}
            >
              Submit Review
            </ButtonPrimary>
          </Box>
        </Box>
      )}
    </Validation>
  );
}

export type Props = {
  submitReview: RequestViewState['submitReview'];
  longTermAccess: RequestViewState['longTermAccess'];
  shortTermDuration: string;
  user: string;
  attempt: Attempt;
  request: AccessRequest;
};

// TODO(lisa): move this to 'shared/ToolTip' package
// and refactor ToolTipInfo with this.
export const ToolTipText: React.FC<
  PropsWithChildren<{
    tipContent: React.ReactElement;
    fontSize?: number;
  }>
> = ({ tipContent, fontSize = 10, children }) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <>
      <span
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
      >
        {children}
      </span>
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
      >
        <StyledOnHover px={2} py={1} fontSize={`${fontSize}px`}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

function makeSuggestedAccessListOptions(
  longTermAccess: LongTermAccess
): SuggestedAcessListOption[] {
  if (!longTermAccess || longTermAccess.error) {
    return [];
  }

  return longTermAccess.suggestedAccessLists.map(a => {
    const traitsMap = a.grants.traits;
    const grantedTraits = Object.keys(traitsMap).map(key =>
      makeTraitLabel(key, traitsMap[key])
    );
    const combinedRolesAndGrants = [...a.grants.roles, ...grantedTraits];

    const $labels = combinedRolesAndGrants.map((label, index) => (
      <TinyLabel
        mr={index === combinedRolesAndGrants.length - 1 ? 0 : 1}
        key={`${label}${index}`}
        kind="secondary"
        title={label}
      >
        {label}
      </TinyLabel>
    ));
    return {
      value: a,
      label: (
        <Box>
          <Text>{a.title}</Text>
          <TextWithSmallerLineHeight>{a.description}</TextWithSmallerLineHeight>
          <Flex alignItems="center">
            <TextMutedNoEllipsis>Grants:</TextMutedNoEllipsis>
            <Flex flexWrap="wrap">{$labels}</Flex>
          </Flex>
        </Box>
      ),
    };
  });
}

function makeReviewStateOptions(
  longTermAccess: LongTermAccess,
  shortTermDuration: string,
  request: AccessRequest
): ReviewStateOption[] {
  // TODO(lisa): teleterm uses the same components, temporary hack
  // to "disable" promoting for teleterm until feature is ready in teleterm.
  if (!longTermAccess) {
    return [
      { value: 'DENIED', label: <>Reject request</> },
      {
        value: 'APPROVED',
        label: (
          <>
            Approve request
            {shortTermDuration ? ` (${shortTermDuration})` : ''}
          </>
        ),
      },
    ];
  }

  const promotedTxt =
    'Approve long-term access via Access List with the requested resources';

  let promotedContent;

  if (longTermAccess.suggestedAccessLists.length > 0) {
    promotedContent = <Text>{promotedTxt}</Text>;
  } else {
    let msg = 'No Access Lists will grant the requested resources';
    if (longTermAccess.error) {
      msg = `Error: ${longTermAccess.error}`;
    } else if (request.resources.length === 0) {
      msg = 'Only supported for resource based access requests';
    }
    promotedContent = (
      <ToolTipText tipContent={<>{msg}</>}>
        <Flex alignItems="center">
          <Text>{promotedTxt}</Text>
          {longTermAccess.error && (
            <Warning color="warning.active" ml={1} size={20} />
          )}
        </Flex>
      </ToolTipText>
    );
  }

  return [
    { value: 'DENIED', label: <>Reject request</> },
    {
      value: 'APPROVED',
      label: (
        <>
          Approve short-term access
          {shortTermDuration ? ` (${shortTermDuration})` : ''}
        </>
      ),
    },
    {
      value: 'PROMOTED',
      disabled:
        !!longTermAccess.error ||
        longTermAccess.suggestedAccessLists.length === 0,
      label: <>{promotedContent}</>,
    },
  ];
}

const TextMutedNoEllipsis = styled.div`
  font-size: ${p => p.theme.fontSizes[0]}px;
  margin-right: ${p => p.theme.space[1]}px;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const TinyLabel = styled(Label)`
  font-size: 8px;
  padding: 0 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 14px;
`;

const TextWithSmallerLineHeight = styled(Text)`
  line-height: 16px;
  font-size: ${p => p.theme.fontSizes[0]}px;
  color: ${p => p.theme.colors.text.muted};
`;

const HorizontalLine = styled.div`
  width: 2px;
  height: 155px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  position: absolute;
  top: -10px;
  left: -10px;
`;
