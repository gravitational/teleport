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

import { Fragment } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonBorder,
  ButtonPrimary,
  Flex,
  H3,
  Indicator,
  Label,
  LabelState,
  Text,
} from 'design';
import Table from 'design/DataTable';
import { displayDateWithPrefixedTime } from 'design/datetime';
import {
  ArrowFatLinesUp,
  ChevronCircleDown,
  CircleCheck,
  CircleCross,
} from 'design/Icon';
import { LabelKind } from 'design/LabelState/LabelState';
import { TeleportGearIcon } from 'design/SVGIcon';
import { HoverTooltip } from 'design/Tooltip';
import { Attempt, hasFinished } from 'shared/hooks/useAsync';
import {
  AccessRequest,
  AccessRequestReview,
  AccessRequestReviewer,
  canAssumeNow,
  RequestState,
  Resource,
} from 'shared/services/accessRequests';

import type {
  RequestFlags,
  SubmitReview,
} from '../../ReviewRequests/RequestView/types';
import {
  getAssumeStartTimeTooltipText,
  PromotedMessage,
} from '../../Shared/Shared';
import { getFormattedDurationTxt } from '../../Shared/utils';
import { formattedName } from '../formattedName';
import { RequestDelete } from './RequestDelete';
import RequestReview from './RequestReview';
import RolesRequested from './RolesRequested';
import { SuggestedAccessList } from './types';

export interface RequestViewProps {
  user: string;
  getFlags(accessRequest: AccessRequest): RequestFlags;
  fetchRequestAttempt: Attempt<AccessRequest>;
  fetchSuggestedAccessListsAttempt: Attempt<SuggestedAccessList[]>;
  toggleConfirmDelete(): void;
  confirmDelete: boolean;
  submitReview(s: SubmitReview): void;
  submitReviewAttempt: Attempt<AccessRequest>;
  assumeRole(accessRequest: AccessRequest): void;
  assumeRoleAttempt: Attempt<void>;
  assumeAccessList(): void;
  deleteRequestAttempt: Attempt<void>;
  deleteRequest(): void;
}

export function RequestView({
  user,
  fetchRequestAttempt,
  getFlags,
  confirmDelete,
  toggleConfirmDelete,
  submitReview,
  assumeRole,
  submitReviewAttempt,
  assumeRoleAttempt,
  fetchSuggestedAccessListsAttempt,
  assumeAccessList,
  deleteRequestAttempt,
  deleteRequest,
}: RequestViewProps) {
  if (
    !hasFinished(fetchRequestAttempt) ||
    !hasFinished(fetchSuggestedAccessListsAttempt)
  ) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator delay="short" />
      </Box>
    );
  }

  if (fetchRequestAttempt.status === 'error') {
    return <Alert kind="danger" children={fetchRequestAttempt.statusText} />;
  }

  if (assumeRoleAttempt.status === 'error') {
    return <Alert kind="danger" children={assumeRoleAttempt.statusText} />;
  }

  const request =
    submitReviewAttempt.status === 'success'
      ? submitReviewAttempt.data
      : fetchRequestAttempt.data;
  const flags = getFlags(request);

  let assumeBtn;
  if (flags.canAssume) {
    if (canAssumeNow(request.assumeStartTime)) {
      assumeBtn = (
        <ButtonPrimary
          disabled={
            flags.isAssumed || assumeRoleAttempt.status === 'processing'
          }
          onClick={() => assumeRole(request)}
          mt={4}
        >
          {flags.isAssumed ? 'Assumed' : 'Assume Roles'}
        </ButtonPrimary>
      );
    } else {
      assumeBtn = (
        <Box mt={4}>
          <HoverTooltip
            tipContent={getAssumeStartTimeTooltipText(request.assumeStartTime)}
            placement="top-start"
          >
            <ButtonPrimary disabled={true}>Assume Roles</ButtonPrimary>
          </HoverTooltip>
        </Box>
      );
    }
  }

  let requestedAccessTime = getFormattedDurationTxt({
    start: request.created,
    end: request.expires,
  });
  let startingTime = displayDateWithPrefixedTime(request.created);
  if (request.assumeStartTime) {
    startingTime = displayDateWithPrefixedTime(request.assumeStartTime);
    requestedAccessTime = getFormattedDurationTxt({
      start: request.assumeStartTime,
      end: request.expires,
    });
  }

  return (
    <>
      {confirmDelete && (
        <RequestDelete
          user={request.user}
          roles={request.roles}
          requestId={request.id}
          requestState={request.state}
          onClose={toggleConfirmDelete}
          onDelete={deleteRequest}
          deleteRequestAttempt={deleteRequestAttempt}
        />
      )}

      <Flex>
        {/* Left box contains: status, timestamps, and comments */}
        <Box
          mr={5}
          width="100%"
          minWidth="515px"
          maxWidth="860px"
          flex="1 1 auto"
        >
          <Box
            css={`
              box-shadow: ${props => props.theme.boxShadow[0]};
            `}
          >
            {/* First half of this box contains status, roles, expiry, and delete btn */}
            <Flex
              p={3}
              borderTopLeftRadius={2}
              borderTopRightRadius={2}
              css={`
                background: ${props =>
                  props.theme.type === 'light'
                    ? props.theme.colors.spotBackground[0]
                    : props.theme.colors.levels.elevated};
              `}
            >
              <Flex alignItems="center">
                <StateLabel
                  state={request.state}
                  mr={3}
                  px={3}
                  py={1}
                  style={{ fontWeight: 'bold' }}
                />
                <H3>
                  <Flex flexWrap="wrap" alignItems="baseline">
                    <Text
                      mr={1}
                      title={request.user}
                      bold
                      style={{
                        maxWidth: '120px',
                      }}
                    >
                      {request.user}
                    </Text>
                    <Text
                      mr={2}
                      typography="body3"
                      style={{
                        flexShrink: 0,
                        whiteSpace: 'nowrap',
                      }}
                    >
                      is requesting roles:
                    </Text>
                    <RolesRequested roles={request.roles} />
                    <Text typography="body3">
                      for {requestedAccessTime}, starting {startingTime}
                    </Text>
                  </Flex>
                </H3>
              </Flex>
              <Flex
                alignItems="baseline"
                justifyContent="flex-end"
                flexWrap="wrap-reverse"
                flex="1"
                gap={2}
              >
                {request.requestTTLDuration && request.state === 'PENDING' && (
                  <RequestTtlLabel typography="body4" ml={1}>
                    Request expires in {request.requestTTLDuration}
                  </RequestTtlLabel>
                )}
                <ButtonBorder
                  disabled={!flags.canDelete}
                  onClick={toggleConfirmDelete}
                  size="small"
                  width="60px"
                >
                  Delete
                </ButtonBorder>
              </Flex>
            </Flex>
            {/* Second half of this box contains timestamp & comments*/}
            <TimelineCommentAndReviewsContainer
              style={{ position: 'relative' }}
            >
              <Timeline />
              <RequestorTimestamp
                user={request.user}
                reason={request.requestReason}
                createdDuration={request.createdDuration}
                resources={request.resources}
              />
              {request.reviews.length > 0 && (
                <Reviews reviews={request.reviews} />
              )}
              {request.state === 'PENDING' &&
                fetchSuggestedAccessListsAttempt.status === 'success' &&
                fetchSuggestedAccessListsAttempt.data.length > 0 && (
                  <SuggestedAccessListTimestamp
                    accessLists={fetchSuggestedAccessListsAttempt.data}
                  />
                )}
              {flags.canReview && (
                <RequestReview
                  submitReview={submitReview}
                  user={user}
                  submitReviewAttempt={submitReviewAttempt}
                  fetchSuggestedAccessListsAttempt={
                    fetchSuggestedAccessListsAttempt
                  }
                  shortTermDuration={requestedAccessTime}
                  request={request}
                />
              )}
            </TimelineCommentAndReviewsContainer>
          </Box>
          {assumeBtn}
          {request.state === 'PROMOTED' && request.promotedAccessListTitle && (
            <PromotedMessage
              request={request}
              self={flags.ownRequest}
              py={4}
              assumeAccessList={assumeAccessList}
            />
          )}
        </Box>
        {/* Right box contains reviewers and threshold list */}
        <Box flex="0 1 260px" minWidth="120px">
          <Reviewers reviewers={request.reviewers} />
          <Box mt={3} ml={1}>
            <Text typography="body3" color="text.slightlyMuted">
              Thresholds: {request.thresholdNames.join(', ')}
            </Text>
          </Box>
        </Box>
      </Flex>
    </>
  );
}

export const Timeline = styled.div`
  position: absolute;
  height: calc(100% - 34px);
  width: 2px;
  top: 0;
  left: 55px;
  border-left: 2px solid ${props => props.theme.colors.spotBackground[0]};
`;

export function RequestorTimestamp({
  user,
  reason,
  createdDuration,
  resources,
}: {
  user: string;
  reason: string;
  createdDuration: string;
  resources: Resource[];
}) {
  return (
    <>
      <Timestamp author={user} createdDuration={createdDuration} />
      {(reason || resources?.length > 0) && (
        <Comment
          author={user}
          comment={reason}
          createdDuration={createdDuration}
          resources={resources}
        />
      )}
    </>
  );
}

export function Timestamp({
  author,
  state,
  createdDuration,
  promotedAccessListTitle,
  assumeStartTime,
}: {
  author: string;
  state?: RequestState;
  createdDuration: string;
  promotedAccessListTitle?: string;
  assumeStartTime?: Date;
}) {
  const isPromoted = state === 'PROMOTED' && promotedAccessListTitle;

  let iconBgColor = 'levels.elevated';
  let $icon = <ChevronCircleDown size={26} color="text.muted" />;
  let verb = `submitted`;

  if (state === 'APPROVED') {
    iconBgColor = 'success.main';
    $icon = <CircleCheck size={26} color="light" />;
    verb = 'approved';
  }

  if (isPromoted) {
    iconBgColor = 'success.main';
    $icon = <ArrowFatLinesUp size={26} color="light" />;
    verb = 'promoted';
  }

  if (state === 'DENIED') {
    iconBgColor = 'error.main';
    $icon = <CircleCross size={26} color="light" />;
    verb = 'denied';
  }

  return (
    <Flex alignItems="center" pt={3} style={{ position: 'relative' }}>
      <Box
        ml={3}
        mr={2}
        bg={iconBgColor}
        p="3px"
        borderRadius="50%"
        style={{ display: 'flex' }}
      >
        {$icon}
      </Box>
      <Text typography="body2">
        <b>{author}</b>{' '}
        {!isPromoted ? (
          assumeStartTime ? (
            <span>
              modified the start time and {verb} this request {createdDuration}
            </span>
          ) : (
            <span>
              {verb} this request {createdDuration}
            </span>
          )
        ) : (
          <span>
            {verb} this request to long-term access with access list{' '}
            <b>{promotedAccessListTitle}</b> {createdDuration}
          </span>
        )}
      </Text>
    </Flex>
  );
}

function Comment({
  author,
  comment,
  createdDuration,
  resources,
}: {
  author: string;
  comment: string;
  createdDuration: string;
  resources?: Resource[];
}) {
  return (
    <Box
      border="1px solid"
      borderColor="levels.sunken"
      mt={3}
      style={{ position: 'relative' }}
    >
      <Flex bg="levels.sunken" py={1} px={3} alignItems="baseline">
        <H3 mr={2}>{author}</H3>
        <Text typography="body3">{createdDuration}</Text>
      </Flex>
      {comment && (
        <Box p={3} bg="levels.elevated">
          {comment}
        </Box>
      )}
      {resources?.length > 0 && (
        <Box
          pt={comment ? 0 : 3}
          pl={3}
          pr={0}
          pb={3}
          css={`
            margin: 0 auto;
          `}
          bg="levels.elevated"
        >
          <StyledTable
            data={resources.map(resource => ({
              ...resource.id,
              ...resource.details,
              name: resource.details?.friendlyName || formattedName(resource),
            }))}
            columns={[
              {
                key: 'clusterName',
                headerText: 'Cluster Name',
              },
              {
                key: 'kind',
                headerText: 'Requested Resource Kind',
              },
              {
                key: 'name',
                headerText: 'Requested Resource Name',
              },
            ]}
            emptyText=""
          />
        </Box>
      )}
    </Box>
  );
}

function Reviewers({ reviewers }: { reviewers: AccessRequestReviewer[] }) {
  const $reviewers = reviewers.map((reviewer, index) => {
    let kind: LabelKind = 'warning';
    if (reviewer.state === 'APPROVED' || reviewer.state === 'PROMOTED') {
      kind = 'success';
    } else if (reviewer.state === 'DENIED') {
      kind = 'danger';
    }

    return (
      <Flex
        border={1}
        borderColor="levels.surface"
        borderRadius={1}
        px={3}
        py={2}
        mb={2}
        alignItems="center"
        justifyContent="space-between"
        key={index}
        css={`
          background: ${props => props.theme.colors.spotBackground[0]};
        `}
      >
        <Text
          typography="body3"
          bold
          mr={3}
          style={{
            whiteSpace: 'nowrap',
            maxWidth: '200px',
          }}
          title={reviewer.name}
        >
          {reviewer.name}
        </Text>
        <LabelState
          kind={kind}
          width="10px"
          p={0}
          style={{
            minHeight: '10px',
            minWidth: '10px',
          }}
        />
      </Flex>
    );
  });

  if ($reviewers.length === 0) {
    return (
      <>
        <Flex
          borderBottom={1}
          mb={3}
          pb={3}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[1]};
          `}
        >
          <H3 mr={2}>No Reviewers Yet</H3>
        </Flex>
        {$reviewers}
      </>
    );
  }

  return (
    <>
      <Flex
        borderBottom={1}
        mb={3}
        pb={3}
        css={`
          border-color: ${props => props.theme.colors.spotBackground[1]};
        `}
      >
        <H3 mr={2}>Reviewers</H3>
      </Flex>
      {$reviewers}
    </>
  );
}

function StateLabel(props: { state: RequestState; [key: string]: any }) {
  const { state, ...styles } = props;
  switch (state) {
    case 'APPROVED':
    case 'PROMOTED':
      return (
        <LabelState kind="success" {...styles}>
          {state}
        </LabelState>
      );
    case 'DENIED':
      return (
        <LabelState kind="danger" {...styles}>
          {state}
        </LabelState>
      );
    case 'PENDING':
      return (
        <LabelState kind="warning" {...styles}>
          {state}
        </LabelState>
      );
  }
}

function Reviews({ reviews }: { reviews: AccessRequestReview[] }) {
  const $reviews = reviews.map((review, index) => {
    const { author, state, createdDuration, reason, promotedAccessListTitle } =
      review;

    return (
      <Fragment key={index}>
        <Timestamp
          author={author}
          state={state}
          createdDuration={createdDuration}
          promotedAccessListTitle={promotedAccessListTitle}
          assumeStartTime={review.assumeStartTime}
        />
        {reason && (
          <Comment
            author={author}
            comment={reason}
            createdDuration={createdDuration}
          />
        )}
      </Fragment>
    );
  });

  return <Box>{$reviews}</Box>;
}

export function SuggestedAccessListTimestamp({
  accessLists,
}: {
  accessLists: SuggestedAccessList[];
}) {
  return (
    <Flex pt={3} style={{ position: 'relative' }}>
      <Box ml={3} mr={2}>
        <TeleportGearIcon size={32} />
      </Box>
      <Box>
        <Text>
          <BrandName>Teleport</BrandName> identified {accessLists.length} access
          lists which grants similar requested resources:
        </Text>
        <Flex gap={2} flexWrap="wrap">
          {accessLists.map(acl => (
            <Label key={acl.id} kind="secondary">
              {acl.title}
            </Label>
          ))}
        </Flex>
      </Box>
    </Flex>
  );
}

const StyledTable = styled(Table)`
  width: 90%;

  & > tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;

const BrandName = styled.span`
  font-weight: bold;
  color: ${p => p.theme.colors.brand};
`;

export const TimelineCommentAndReviewsContainer = styled.div`
  position: relative;
  background-color: ${p => p.theme.colors.levels.surface};
  padding: ${p => p.theme.space[4]}px;
  padding-top: 0;
  border-bottom-left-radius: ${p => p.theme.radii[4]}px;
  border-bottom-right-radius: ${p => p.theme.radii[4]}px;
`;

const RequestTtlLabel = styled(Text)`
  font-style: italic;
`;
