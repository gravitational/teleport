/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { ButtonPrimary } from 'design';
import Flex from 'design/Flex';

import { useAttemptNext } from 'shared/hooks';
import Select from 'shared/components/Select';

import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  Resources,
  Roles,
} from 'teleport/Assist/Conversation/AccessRequests/Resources+Roles';
import {
  createAccessRequest,
  getDurationOptions,
} from 'teleport/AccessRequests/service';

import { useAssist } from 'teleport/Assist/context/AssistContext';

import type { DurationOption } from 'teleport/AccessRequests/types';
import type { AccessRequestResource } from 'teleport/Assist/types';

interface AccessRequestProps {
  resources: AccessRequestResource[];
  roles: string[];
  reason: string;
}

const Container = styled.div`
  padding: 15px 15px 15px 17px;
`;

const StyledInput = styled.input<{ hasError: boolean }>`
  border: 1px solid
    ${p =>
      p.hasError
        ? p.theme.colors.error.main
        : p.theme.colors.spotBackground[0]};
  padding: 12px 15px;
  border-radius: 5px;
  font-family: ${p => p.theme.font};
  background: ${p => p.theme.colors.levels.surface};
  width: 300px;

  &:disabled {
    background: ${p => p.theme.colors.spotBackground[0]};
  }

  &:active:not(:disabled),
  &:focus:not(:disabled) {
    outline: none;
    border-color: ${p => p.theme.colors.text.slightlyMuted};
  }
`;

const InfoText = styled.span`
  display: block;
  font-size: 14px;
  font-weight: 600;
  margin: 5px 0;
`;

const SubTitle = styled.div`
  font-size: 13px;
  font-weight: 600;
  margin: 5px 0;
`;

const Padding = styled.div`
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
`;

const ErrorMessage = styled.div`
  color: ${p => p.theme.colors.error.main};
  margin-top: ${p => p.theme.space[2]}px;
`;

export function AccessRequest(props: AccessRequestProps) {
  const { clusterId } = useStickyClusterId();

  const { sendAccessRequestCreatedMessage } = useAssist();

  const [reason, setReason] = useState(props.reason);

  const [durationOptions, setDurationOptions] = useState<DurationOption[]>([]);
  const [maxDuration, setMaxDuration] = useState<DurationOption>({
    value: 0,
    label: '',
  });

  const duration = useAttemptNext('processing');
  const creating = useAttemptNext();

  useEffect(() => {
    async function init() {
      const durationOptions = await getDurationOptions(
        clusterId,
        props.roles,
        props.resources
      );

      setDurationOptions(durationOptions);
    }

    duration.run(init);
  }, []);

  function handleDurationChange(option: DurationOption) {
    setMaxDuration(option);
  }

  async function create() {
    creating.setAttempt({ status: 'processing' });

    try {
      const maxDurationValue =
        maxDuration.value === 0 ? null : new Date(maxDuration.value);

      const accessRequest = await createAccessRequest(
        clusterId,
        props.roles,
        props.resources,
        reason,
        false,
        maxDurationValue
      );

      sendAccessRequestCreatedMessage(accessRequest.id);
    } catch (err) {
      creating.setAttempt({ status: 'failed', statusText: err.message });
    }
  }

  if (duration.attempt.status === 'failed') {
    return (
      <Padding>An error occurred loading the access request form.</Padding>
    );
  }

  if (duration.attempt.status === 'processing') {
    return <Padding>Loading...</Padding>;
  }

  return (
    <Container>
      <InfoText style={{ marginTop: 0 }}>Create an access request</InfoText>

      <SubTitle>Resources</SubTitle>

      {props.resources.length > 0 && <Resources resources={props.resources} />}

      <SubTitle>Roles</SubTitle>

      {props.roles && props.roles.length > 0 && <Roles roles={props.roles} />}

      <SubTitle>Reason</SubTitle>

      <StyledInput value={reason} onChange={e => setReason(e.target.value)} />

      {durationOptions.length > 0 && (
        <>
          <SubTitle>Max Duration</SubTitle>

          <Select
            onChange={handleDurationChange}
            value={maxDuration}
            options={durationOptions}
          />
        </>
      )}

      <Flex mt={3} justifyContent="flex-end">
        <ButtonPrimary
          ml={3}
          onClick={create}
          disabled={creating.attempt.status === 'processing'}
        >
          {creating.attempt.status === 'processing' ? 'Creating...' : 'Create'}
        </ButtonPrimary>
      </Flex>

      {creating.attempt.status === 'failed' && (
        <ErrorMessage>
          An error occurred creating the access request.
        </ErrorMessage>
      )}
    </Container>
  );
}
