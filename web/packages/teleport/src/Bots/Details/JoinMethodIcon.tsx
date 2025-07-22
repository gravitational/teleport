/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { IconProps } from 'design/Icon/Icon';
import { AmazonAws } from 'design/Icon/Icons/AmazonAws';
import { GitHub } from 'design/Icon/Icons/GitHub';
import { Key } from 'design/Icon/Icons/Key';
import { Keypair } from 'design/Icon/Icons/Keypair';
import { Kubernetes } from 'design/Icon/Icons/Kubernetes';
import { Memory } from 'design/Icon/Icons/Memory';
import { UserCheck } from 'design/Icon/Icons/UserCheck';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

export function JoinMethodIcon(props: {
  method: string | undefined | null;
  size?: IconProps['size'];
  color?: IconProps['color'];
  includeTooltip?: boolean;
}) {
  const { method, size, color, includeTooltip = true } = props;
  if (!method) return null;
  const Icon = iconForMethod(method);
  return includeTooltip ? (
    <HoverTooltip placement="top" tipContent={method}>
      <Icon size={size} color={color} />
    </HoverTooltip>
  ) : (
    <Icon size={size} color={color} />
  );
}

// TODO(nicholasmarais1158): Add missing icons once designed
const iconForMethod = (method: string) => {
  switch (method) {
    case 'token':
      return Key;
    case 'ec2':
      return AmazonAws;
    case 'iam':
      return UserCheck;
    case 'github':
      return GitHub;
    case 'circleci':
      return Key; // Needs an icon created
    case 'kubernetes':
      return Kubernetes;
    case 'azure':
      return Key; // Needs an icon created
    case 'gitlab':
      return Key; // Needs an icon created
    case 'gcp':
      return Key; // Needs an icon created
    case 'spacelift':
      return Key; // Needs an icon created
    case 'tpm':
      return Memory;
    case 'terraform_cloud':
      return Key; // Needs an icon created
    case 'bitbucket':
      return Key; // Needs an icon created
    case 'oracle':
      return Key; // Needs an icon created
    case 'azure_devops':
      return Key; // Needs an icon created
    case 'bound_keypair':
      return Keypair;
    default:
      return Key;
  }
};
