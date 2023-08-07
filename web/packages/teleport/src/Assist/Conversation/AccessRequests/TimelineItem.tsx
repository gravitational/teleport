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

import React from 'react';
import styled from 'styled-components';

import { formatRelative } from 'date-fns';

import { Resources } from 'teleport/Assist/Conversation/AccessRequests/Resources';

import type { AccessRequestEvent } from 'teleport/Assist/types';

interface TimelineItemProps {
  event: AccessRequestEvent;
}

const Container = styled.div`
  display: flex;
  margin-left: 20px;
`;

const Timestamp = styled.div`
  font-size: 12px;
  position: absolute;
  right: 0;
  margin-left: 10px;
  color: ${p => p.theme.colors.text.muted};
`;

const Description = styled.div`
  font-size: 14px;
  padding: 10px 0;
`;

const Header = styled.div`
  display: flex;
  position: relative;
  align-items: center;
  padding-right: 70px;

  &:after {
    content: '';
    position: absolute;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: ${p => p.theme.colors.warning.main};
    top: 7px;
    left: -20px;
  }
`;

const Username = styled.div`
  font-weight: bold;
  margin-right: 4px;
`;

const Role = styled.div`
  background: ${p => p.theme.colors.spotBackground[0]};
  border: 1px solid ${p => p.theme.colors.spotBackground[0]};
  padding: 3px 4px;
  border-radius: 7px;
  font-size: 13px;
  line-height: 1;
  display: flex;
`;

const Roles = styled.div`
  margin-left: 5px;
  display: flex;
  align-items: center;
`;

export function TimelineItem(props: TimelineItemProps) {
  return (
    <Container>
      <div>
        <Header>
          <Username>{props.event.username}</Username>is requesting the{' '}
          {props.event.roles.length > 1 ? 'roles' : 'role'}
          <Roles>
            {props.event.roles.map((role, index) => (
              <Role key={index}>{role}</Role>
            ))}
          </Roles>
          <Timestamp>{formatDate(props.event.created)}</Timestamp>
        </Header>

        <Description>{props.event.message}</Description>

        <Resources resources={props.event.resources} />
      </div>
    </Container>
  );
}

function formatDate(date: Date) {
  const now = Date.now();
  const compare = date.getTime();

  if (now - compare < 1000 * 60) {
    return 'just now';
  }

  const minutes = Math.floor((now - compare) / 60000);

  if (minutes === 1) {
    return '1m ago';
  }

  if (minutes > 59 && minutes < 120) {
    return '1h ago';
  }

  if (minutes >= 120) {
    const hours = Math.floor(minutes / 60);

    if (hours >= 24) {
      return formatRelative(date, Date.now());
    }

    return `${hours}h ago`;
  }

  return `${minutes}m ago`;
}
