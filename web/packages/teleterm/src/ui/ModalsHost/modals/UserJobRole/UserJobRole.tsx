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

import React, { useRef, useState } from 'react';
import styled from 'styled-components';
import {
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  Input,
  Text,
} from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Close } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';

interface UserJobRoleProps {
  onCancel(): void;

  onSend(jobRole: string): void;
}

const JOB_OPTIONS = [
  'Software Engineer',
  'Support Engineer',
  'DevOps Engineer',
  'Solutions Architect',
  'System Administrator',
];

const OTHER_JOB_ROLE = 'Other';

export function UserJobRole(props: UserJobRoleProps) {
  const inputRef = useRef<HTMLInputElement>();
  const [jobRole, setJobRole] = useState<string | null>(null);
  const [otherJobRole, setOtherJobRole] = useState('');

  const jobRoleToSubmit = jobRole === OTHER_JOB_ROLE ? otherJobRole : jobRole;

  function handleRadioGroupChange(selectedRole: string): void {
    setJobRole(selectedRole);
    if (selectedRole === OTHER_JOB_ROLE) {
      inputRef.current.focus();
    }
  }

  function selectOtherJobRoleOption(): void {
    setJobRole(OTHER_JOB_ROLE);
  }

  function send(): void {
    props.onSend(jobRoleToSubmit);
  }

  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          send();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={1}
          alignItems="baseline"
        >
          <Text typography="h4" bold>
            What describes your current job role best?
          </Text>
          <ButtonIcon
            type="button"
            onClick={props.onCancel}
            color="text.slightlyMuted"
          >
            <Close fontSize={5} />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={3}>
          <RadioGroup
            autoFocus={true}
            name="jobRole"
            options={[...JOB_OPTIONS, OTHER_JOB_ROLE]}
            value={jobRole}
            onChange={handleRadioGroupChange}
          />
          <DarkInput
            ref={inputRef}
            value={otherJobRole}
            onClick={() => {
              selectOtherJobRoleOption();
            }}
            onChange={e => {
              selectOtherJobRoleOption();
              setOtherJobRole(e.target.value);
            }}
            placeholder="Other role…"
            mt={1}
          />
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary mr={3} type="submit" disabled={!jobRoleToSubmit}>
            Send
          </ButtonPrimary>
          <ButtonSecondary type="button" onClick={props.onCancel}>
            Skip
          </ButtonSecondary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}

// TODO(gzdunek): remove after improving inputs styling in Connect
const DarkInput = styled(Input)`
  background: inherit;
  border: 1px ${props => props.theme.colors.action.disabledBackground} solid;
  box-shadow: none;
  color: ${props => props.theme.colors.text.main};
  margin-bottom: 10px;
  font-size: 14px;
  height: 34px;
  transition: border 300ms ease-out;

  ::placeholder {
    opacity: 1;
    color: ${props => props.theme.colors.text.slightlyMuted};
  }
`;
