import React, { useState } from 'react';
import styled from 'styled-components';
import { ButtonPrimary, Text, Box, Alert, Flex, Label } from 'design';
import { Warning } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';
import Validation, { Validator } from 'shared/components/Validation';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAsync';
import { requiredField } from 'shared/components/Validation/rules';
import { HoverTooltip } from 'shared/components/ToolTip';
import { FieldTextArea } from 'shared/components/FieldTextArea';

import { AccessRequest, RequestState } from 'e-teleport/services/workflow';
import { makeTraitLabel } from 'e-teleport/AccessListManagement/Traits';

import { SuggestedAccessList, SubmitReview } from '../types';

type ReviewStateOption = Option<RequestState, React.ReactElement> & {
  disabled?: boolean;
};

type SuggestedAcessListOption = Option<SuggestedAccessList, React.ReactElement>;

export interface RequestReviewProps {
  submitReview(s: SubmitReview): void;
  fetchSuggestedAccessListsAttempt: Attempt<SuggestedAccessList[]>;
  shortTermDuration: string;
  user: string;
  submitReviewAttempt: Attempt<AccessRequest>;
  request: AccessRequest;
}

export default function RequestReview({
  submitReviewAttempt,
  submitReview,
  user,
  fetchSuggestedAccessListsAttempt,
  shortTermDuration,
  request,
}: RequestReviewProps) {
  const [reviewStateOptions] = useState<ReviewStateOption[]>(() =>
    makeReviewStateOptions(
      fetchSuggestedAccessListsAttempt,
      shortTermDuration,
      request
    )
  );

  const [suggestedAccessListOptions] = useState<SuggestedAcessListOption[]>(
    () => makeSuggestedAccessListOptions(fetchSuggestedAccessListsAttempt)
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
  if (submitReviewAttempt.status === 'success') {
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
            {submitReviewAttempt.status === 'error' && (
              <Alert kind="danger" children={submitReviewAttempt.statusText} />
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
                    maxWidth="600px"
                    label={`Select a suggested Access List to add ${request.user} as a member to:`}
                    rule={requiredField('Required')}
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
            <FieldTextArea
              label="Message"
              placeholder="Optional message..."
              value={reason}
              mb={4}
              maxWidth="500px"
              textAreaCss={`
                  font-size: 14px;
                  min-height: 100px;
                `}
              onChange={e => setReason(e.target.value)}
            />
            <ButtonPrimary
              disabled={submitReviewAttempt.status === 'processing'}
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

function makeSuggestedAccessListOptions(
  fetchSuggestedAccessListsAttempt: Attempt<SuggestedAccessList[]>
): SuggestedAcessListOption[] {
  if (fetchSuggestedAccessListsAttempt.status !== 'success') {
    return [];
  }

  return fetchSuggestedAccessListsAttempt.data.map(a => {
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
  fetchSuggestedAccessListsAttempt: Attempt<SuggestedAccessList[]>,
  shortTermDuration: string,
  request: AccessRequest
): ReviewStateOption[] {
  const promotedTxt =
    'Approve long-term access via Access List with the requested resources';

  let promotedContent;

  if (
    fetchSuggestedAccessListsAttempt.status === 'success' &&
    fetchSuggestedAccessListsAttempt.data.length > 0
  ) {
    promotedContent = <Text>{promotedTxt}</Text>;
  } else {
    let msg = 'No Access Lists will grant the requested resources';
    if (fetchSuggestedAccessListsAttempt.status === 'error') {
      msg = fetchSuggestedAccessListsAttempt.statusText;
    } else if (request.resources.length === 0) {
      msg = 'Only supported for resource based access requests';
    }
    promotedContent = (
      <HoverTooltip tipContent={msg}>
        <Flex alignItems="center">
          <Text>{promotedTxt}</Text>
          {fetchSuggestedAccessListsAttempt.status === 'error' && (
            <Warning color="warning.active" ml={1} size={20} />
          )}
        </Flex>
      </HoverTooltip>
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
        fetchSuggestedAccessListsAttempt.status === 'error' ||
        (fetchSuggestedAccessListsAttempt.status === 'success' &&
          fetchSuggestedAccessListsAttempt.data.length === 0),
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
