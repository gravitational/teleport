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

import { Link, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';

export function RoleArnInput({
  description,
  roleName,
  roleArn,
  setRoleArn,
  disabled,
}: {
  description?: React.ReactNode;
  roleName: string;
  roleArn: string;
  setRoleArn: (arn: string) => void;
  disabled: boolean;
}) {
  return (
    <>
      {description || (
        <Text>
          Once Teleport completes setting up OIDC identity provider and creating
          a role named "{roleName}" in AWS cloud shell (step 2), go to your{' '}
          <Link
            target="_blank"
            href={`https://console.aws.amazon.com/iamv2/home#/roles/details/${roleName}`}
          >
            IAM Role dashboard
          </Link>{' '}
          and copy and paste the role ARN below. Teleport will use this role to
          identity itself to AWS.
        </Text>
      )}
      <FieldInput
        mt={3}
        rule={requiredRoleArn(roleName)}
        value={roleArn}
        label="Role ARN (Amazon Resource Name)"
        placeholder={`arn:aws:iam::123456789012:role/${roleName}`}
        width="500px"
        onChange={e => setRoleArn(e.target.value)}
        disabled={disabled}
        toolTipContent={`Unique AWS resource identifier and uses the format: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`}
      />
    </>
  );
}

const requiredRoleArn = (roleName: string) => (roleArn: string) => () => {
  const regex = new RegExp(
    '^arn:aws.*:iam::\\d{12}:role\\/(' + roleName + ')$'
  );

  if (regex.test(roleArn)) {
    return {
      valid: true,
    };
  }

  return {
    valid: false,
    message:
      'invalid role ARN, double check you copied and pasted the correct output',
  };
};
