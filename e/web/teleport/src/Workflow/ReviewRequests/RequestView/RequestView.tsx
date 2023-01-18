import React, { useState } from 'react';
import styled from 'styled-components';
import {
  ButtonPrimary,
  ButtonBorder,
  Text,
  Alert,
  Box,
  Flex,
  LabelState,
  Indicator,
} from 'design';
import { CircleCheck, CircleCross, ChevronCircleDown } from 'design/Icon';
import Table from 'design/DataTable';
import { PrivateKeyAccessRequestDialogue } from '@gravitational/teleport/src/components/PrivateKeyPolicy';

import useTeleportE from 'e-teleport/useTeleportE';
import {
  RequestState,
  AccessRequestReview,
  AccessRequestReviewer,
  Resource,
} from 'e-teleport/services/workflow';

import RequestDelete from './RequestDelete';
import RequestReview from './RequestReview';
import RolesRequested from './RolesRequested';
import useRequestView, { State } from './useRequestView';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRequestView(ctx);
  return <RequestView {...state} />;
}

export function RequestView({
  user,
  attempt,
  request,
  flags,
  confirmDelete,
  toggleConfirmDelete,
  submitReview,
  assumeRole,
  reviewAttempt,
  privateKeyRequirement,
  clearPrivateKeyRequirement,
}: State) {
  // Show indicator as soon as user clicks assume button.
  const [delayIndicator, setDelayIndicator] = useState(true);

  function onAssumeRole() {
    setDelayIndicator(false);
    assumeRole();
  }

  if (attempt.status === 'processing') {
    return (
      <Box textAlign="center" m={10}>
        <Indicator delay={delayIndicator ? 'short' : 'none'} />
      </Box>
    );
  }

  if (attempt.status === 'failed') {
    return <Alert kind="danger" children={attempt.statusText} />;
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
          {/* First half of this box contains status, roles, expiry, and delete btn */}
          <Flex
            bg="primary.lighter"
            p={3}
            borderTopLeftRadius={2}
            borderTopRightRadius={2}
          >
            <Flex alignItems="center">
              <StateLabel
                state={request.state}
                mr={3}
                px={3}
                py={1}
                style={{ fontWeight: 'bold' }}
              />
              <Flex flexWrap="wrap" mb={1}>
                <Flex mt={1} alignItems="center">
                  <Text
                    mr={1}
                    typography="body2"
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
                    typography="body2"
                    style={{
                      flexShrink: 0,
                      whiteSpace: 'nowrap',
                    }}
                  >
                    is requesting roles:
                  </Text>
                </Flex>
                <RolesRequested roles={request.roles} />
              </Flex>
            </Flex>
            <Flex
              alignItems="center"
              justifyContent="flex-end"
              flexWrap="wrap-reverse"
              flex="1"
            >
              <Text typography="body2" style={{ whiteSpace: 'nowrap' }}>
                (expires in {request.expiresDuration})
              </Text>
              <ButtonBorder
                disabled={!flags.canDelete}
                onClick={toggleConfirmDelete}
                size="small"
                width="60px"
                ml={3}
              >
                Delete
              </ButtonBorder>
            </Flex>
          </Flex>
          {/* Second half of this box contains timestamp & comments*/}
          <Box
            bg="primary.light"
            p={4}
            pt={0}
            borderBottomLeftRadius={2}
            borderBottomRightRadius={2}
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
            {flags.canReview && (
              <RequestReview
                submitReview={submitReview}
                user={user}
                attempt={reviewAttempt}
              />
            )}
          </Box>
          {flags.canAssume && (
            <ButtonPrimary
              disabled={flags.isAssumed}
              onClick={onAssumeRole}
              mt={4}
            >
              {flags.isAssumed ? 'assumed' : 'assume roles'}
            </ButtonPrimary>
          )}
        </Box>
        {/* Right box contains reviewers and threshold list */}
        <Box flex="0 1 260px" minWidth="120px">
          <Reviewers reviewers={request.reviewers} />
          <Box mt={3} ml={1}>
            <Text typography="body2" color="text.secondary">
              Thresholds: {request.thresholdNames.join(', ')}
            </Text>
          </Box>
        </Box>
      </Flex>
      {privateKeyRequirement && (
        <PrivateKeyAccessRequestDialogue
          onClose={clearPrivateKeyRequirement}
          {...privateKeyRequirement}
        />
      )}
    </>
  );
}

const Timeline = styled.div`
  position: absolute;
  height: calc(100% - 24px);
  width: 2px;
  top: 0;
  left: 55px;
  border-left: 2px solid ${props => props.theme.colors.primary.lighter};
`;

function RequestorTimestamp({
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

function Timestamp({
  author,
  state,
  createdDuration,
}: {
  author: string;
  state?: RequestState;
  createdDuration: string;
}) {
  let iconBgColor = 'primary.lighter';
  let $icon = <ChevronCircleDown fontSize={8} color="text.placeholder" />;
  let verb = `submitted`;

  if (state === 'APPROVED') {
    iconBgColor = 'success';
    $icon = <CircleCheck fontSize={8} color="light" />;
    verb = 'approved';
  }

  if (state === 'DENIED') {
    iconBgColor = 'danger';
    $icon = <CircleCross fontSize={8} color="light" />;
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
      <Flex alignItems="baseline">
        <Text typography="body2" bold mr={2} style={{ flex: '1 1 0' }}>
          {author}
        </Text>
        <Text typography="body2" alignSelf="end">
          {verb} this request {createdDuration}
        </Text>
      </Flex>
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
      borderColor="primary.dark"
      mt={3}
      style={{ position: 'relative' }}
    >
      <Flex bg="primary.dark" py={1} px={3} alignItems="baseline">
        <Text typography="body2" bold mr={2}>
          {author}
        </Text>
        <Text typography="paragraph2">{createdDuration}</Text>
      </Flex>
      {comment && (
        <Box p={3} bg="primary.lighter">
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
          bg="primary.lighter"
        >
          <StyledTable
            data={resources.map(resource => ({
              ...resource.id,
              ...resource.details,
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
                headerText: 'Requested Resource ID',
              },
              {
                key: 'hostname',
                headerText: 'Hostname',
                // Don't render the hostname column if we have no hostnames.
                isNonRender: !resources.some(
                  resource => resource.details?.hostname
                ),
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
    let kind = 'warning';
    if (reviewer.state === 'APPROVED') {
      kind = 'success';
    } else if (reviewer.state === 'DENIED') {
      kind = 'danger';
    }

    return (
      <Flex
        border={1}
        borderColor="primary.light"
        borderRadius={1}
        px={3}
        py={2}
        mb={2}
        bg="primary.main"
        alignItems="center"
        justifyContent="space-between"
        key={index}
      >
        <Text
          typography="body2"
          bold
          mr={3}
          style={{ whiteSpace: 'nowrap', maxWidth: '200px' }}
          title={reviewer.name}
        >
          {reviewer.name}
        </Text>
        <LabelState
          kind={kind}
          width="10px"
          p={0}
          style={{ minHeight: '10px', minWidth: '10px' }}
        />
      </Flex>
    );
  });

  if ($reviewers.length === 0) {
    return (
      <>
        <Flex borderBottom={1} borderColor="primary.main" mb={3} pb={3}>
          <Text typography="h6" mr={2}>
            No Reviewers Yet
          </Text>
        </Flex>
        {$reviewers}
      </>
    );
  }

  return (
    <>
      <Flex borderBottom={1} borderColor="primary.main" mb={3} pb={3}>
        <Text typography="h6" mr={2}>
          Reviewers
        </Text>
      </Flex>
      {$reviewers}
    </>
  );
}

function StateLabel(props: { state: RequestState; [key: string]: any }) {
  const { state, ...styles } = props;
  switch (state) {
    case 'APPROVED':
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
    default:
      return (
        <LabelState kind="warning" {...styles}>
          {state}
        </LabelState>
      );
  }
}

function Reviews({ reviews }: { reviews: AccessRequestReview[] }) {
  const $reviews = reviews.map((review, index) => {
    const { author, state, createdDuration, reason } = review;

    return (
      <React.Fragment key={index}>
        <Timestamp
          author={author}
          state={state}
          createdDuration={createdDuration}
        />
        {reason && (
          <Comment
            author={author}
            comment={reason}
            createdDuration={createdDuration}
          />
        )}
      </React.Fragment>
    );
  });

  return <Box>{$reviews}</Box>;
}

const StyledTable = styled(Table)`
  width: 90%;
  & > tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;
