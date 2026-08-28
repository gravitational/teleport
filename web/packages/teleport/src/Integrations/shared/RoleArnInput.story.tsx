/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { useState } from 'react';

import { ButtonSecondary } from 'design/Button';
import Validation from 'shared/components/Validation';

import { StyledBox } from 'teleport/Discover/Shared';

import { RoleArnInput } from './RoleArnInput';

export default {
  title: 'Teleport/Integrations/Shared/AwsOidc/RoleArnInput',
};

export const Enabled = () => {
  const [roleArn, setRoleArn] = useState('');
  return (
    <Validation>
      <StyledBox width={700}>
        <RoleArnInput
          roleName="test-role"
          roleArn={roleArn}
          setRoleArn={setRoleArn}
          disabled={false}
        />
      </StyledBox>
    </Validation>
  );
};

export const Disabled = () => {
  const [roleArn, setRoleArn] = useState(
    'arn:aws:iam::1234567890:role/test-role'
  );
  return (
    <Validation>
      <StyledBox width={700}>
        <RoleArnInput
          roleName="test-role"
          roleArn={roleArn}
          setRoleArn={setRoleArn}
          disabled={true}
        />
      </StyledBox>
    </Validation>
  );
};

export const Error = () => {
  const [roleArn, setRoleArn] = useState('');
  return (
    <Validation>
      {({ validator }) => (
        <>
          <StyledBox width={700}>
            <RoleArnInput
              roleName="test-role"
              roleArn={roleArn}
              setRoleArn={setRoleArn}
              disabled={false}
            />
          </StyledBox>
          <ButtonSecondary
            mt={6}
            onClick={() => {
              if (!validator.validate()) {
                return;
              }
            }}
          >
            Test Validation
          </ButtonSecondary>
        </>
      )}
    </Validation>
  );
};
