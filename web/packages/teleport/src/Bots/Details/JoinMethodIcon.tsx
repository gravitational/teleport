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
import { Key } from 'design/Icon/Icons/Key';
import { Keypair } from 'design/Icon/Icons/Keypair';
import { Memory } from 'design/Icon/Icons/Memory';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

export function JoinMethodIcon(props: {
  method: string | undefined | null;
  size?: IconProps['size'];
  color?: IconProps['color'];
  includeTooltip?: boolean;
}) {
  const { method, size, color, includeTooltip = true } = props;
  if (!method) return null;

  const icon = renderLogo(method, size) ?? renderIcon(method, size, color);

  return includeTooltip ? (
    <HoverTooltip placement="top" tipContent={method}>
      {icon}
    </HoverTooltip>
  ) : (
    icon
  );
}

const renderLogo = (method: string, size: IconProps['size']) => {
  const name = (() => {
    switch (method) {
      case 'ec2':
        return 'ec2';
      case 'iam':
        return 'awsaccount';
      case 'github':
        return 'github';
      case 'circleci':
        return 'circleci';
      case 'kubernetes':
        return 'kube';
      case 'azure':
        return 'azure';
      case 'gitlab':
        return 'gitlab';
      case 'gcp':
        return 'googlecloud';
      case 'spacelift':
        return 'spacelift';
      case 'terraform_cloud':
        return 'terraform';
      case 'bitbucket':
        return 'git';
      case 'oracle':
        return 'oracle';
      case 'azure_devops':
        return 'azure';
    }
  })();

  return name ? <ResourceIcon name={name} size={size} /> : undefined;
};

const renderIcon = (
  method: string,
  size: IconProps['size'],
  color: IconProps['color']
) => {
  switch (method) {
    case 'token':
      return <Key size={size} color={color} />;
    case 'tpm':
      return <Memory size={size} color={color} />;
    case 'bound_keypair':
      return <Keypair size={size} color={color} />;
    default:
      return <Key size={size} color={color} />;
  }
};
