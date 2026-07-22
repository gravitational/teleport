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

import { useRef, useState } from 'react';
import styled from 'styled-components';

import { ButtonIcon, ButtonPrimary, ButtonSecondary, H2, Input } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';

const JOB_OPTIONS = [
  'Software Engineer',
  'Support Engineer',
  'DevOps Engineer',
  'Solutions Architect',
  'System Administrator',
];

const OTHER_JOB_ROLE = 'Other';

export function UserJobRole(props: {
  onCancel(): void;
  onSend(jobRole: string): void;
  hidden?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
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
      open={!props.hidden}
      keepInDOMAfterClose
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
          <H2 mb={4}>What describes your current job role best?</H2>
          <ButtonIcon
            type="button"
            onClick={props.onCancel}
            color="text.slightlyMuted"
          >
            <Cross size="medium" />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={3}>
          <RadioGroup
            autoFocus={true}
            name="jobRole"
            options={[...JOB_OPTIONS, OTHER_JOB_ROLE]}
            value={jobRole}
            onChange={handleRadioGroupChange}
            mb={3}
          />
          <StyledInput
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

const StyledInput = styled(Input)`
  margin-bottom: 10px;
  font-size: 14px;
  height: 34px;
`;
