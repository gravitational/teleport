/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import styled from 'styled-components';
import Flex from 'design/Flex';
import { ButtonPrimary, ButtonSecondary } from 'design';

import { AccessRequestEvent, AccessRequestStatus } from 'teleport/Assist/types';
import { TimelineItem } from 'teleport/Assist/Conversation/AccessRequests/TimelineItem';

interface AccessRequestsProps {
  status: AccessRequestStatus;
  summary: string;
  username: string;
  events: AccessRequestEvent[];
  created: Date;
}

const ExpandButton = styled.button`
  border: none;
  position: absolute;
  background: none;
  font-family: ${p => p.theme.font};
  left: 50%;
  transform: translateX(-50%);
  bottom: 10px;
  cursor: pointer;
  font-size: 12px;
  color: ${p => p.theme.colors.text.main};
  opacity: 0.8;
  display: flex;
  align-items: center;
  gap: 5px;
  z-index: 3;
  text-shadow: 0 0 2px black;
`;

const Container = styled.div`
  padding: 10px 15px;
  width: 100%;
  box-sizing: border-box;
  max-height: ${p => (p.expanded ? '1000px' : '109px')};
  overflow: hidden;
  position: relative;
  cursor: ${p => (p.expanded ? 'auto' : 'pointer')};
  transition: max-height 0.3s ease-in-out;

  &:hover {
    ${ExpandButton} {
      opacity: 1;
    }
  }

  &:after {
    display: ${p => (p.expanded ? 'none' : 'block')};
    content: '';
    position: absolute;
    bottom: 0;
    left: 10px;
    right: 10px;
    top: 0;
    background: linear-gradient(transparent, #4a5688);
  }
`;

const Overview = styled.div`
  padding-bottom: 10px;
  margin-bottom: 10px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

const Timeline = styled.div`
  position: relative;
`;

const TimelineTrack = styled.div`
  position: absolute;
  width: 2px;
  background: ${p => p.theme.colors.spotBackground[0]};
  left: 4px;
  top: 10px;
  bottom: 30px;
`;

const CommentBox = styled.div`
  margin-top: 20px;
  margin-left: 20px;
  position: relative;

  &:after {
    content: '';
    position: absolute;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: ${p => p.theme.colors.brand};
    top: 15px;
    left: -20px;
  }
`;

const Textarea = styled.textarea`
  width: 100%;
  box-sizing: border-box;
  background: ${p => p.theme.colors.levels.popout};
  color: ${p => p.theme.colors.text.main};
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 10px;
  resize: none;
  padding: 12px 15px;
  height: 40px;
  line-height: 1;
  font-size: 13px;

  &:focus {
    outline: none;
    border-color: ${p => p.theme.colors.spotBackground[2]};
  }

  ::placeholder {
    color: ${p => p.theme.colors.text.muted};
  }
`;

export function AccessRequests(props: AccessRequestsProps) {
  const [expanded, setExpanded] = useState(false);

  function handleExpand() {
    if (expanded) {
      return;
    }

    setExpanded(true);
  }

  const events = props.events.map((event, index) => (
    <TimelineItem key={index} event={event} />
  ));

  return (
    <Container expanded={expanded} onClick={handleExpand}>
      <Overview>
        <strong>{props.username}</strong> {props.summary}
      </Overview>
      <Timeline>
        <TimelineTrack />

        {events}

        <CommentBox>
          <Textarea placeholder="Add an optional comment" />
        </CommentBox>
      </Timeline>

      <Flex justifyContent="flex-end">
        <ButtonSecondary>Decline</ButtonSecondary>
        <ButtonPrimary ml={2}>Approve</ButtonPrimary>
      </Flex>

      {!expanded && (
        <ExpandButton onClick={() => setExpanded(true)}>
          Show more details
        </ExpandButton>
      )}
    </Container>
  );
}
