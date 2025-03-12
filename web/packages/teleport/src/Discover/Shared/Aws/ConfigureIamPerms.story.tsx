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

import { ConfigureIamPerms } from './ConfigureIamPerms';

export default {
  title: 'Teleport/Discover/Shared/ConfigureIamPerms',
};

export const Ec2 = () => {
  return (
    <ConfigureIamPerms
      kind="ec2"
      region="us-east-1"
      integrationRoleArn="arn:aws:iam::123456789012:role/some-iam-role-name"
    />
  );
};

export const Eks = () => {
  return (
    <ConfigureIamPerms
      kind="eks"
      region="us-east-1"
      integrationRoleArn="arn:aws:iam::123456789012:role/some-iam-role-name"
    />
  );
};

export const Rds = () => {
  return (
    <ConfigureIamPerms
      kind="rds"
      region="us-east-1"
      integrationRoleArn="arn:aws:iam::123456789012:role/some-iam-role-name"
    />
  );
};
