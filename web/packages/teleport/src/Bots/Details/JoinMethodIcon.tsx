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

import { ReactElement } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
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
  let logo: ReactElement | null = null;

  switch (method) {
    case 'ec2':
      logo = <ResourceIcon name={'ec2'} width={sizetoInnerPx(size)} />;
      break;
    case 'iam':
      logo = <ResourceIcon name={'awsaccount'} width={sizetoInnerPx(size)} />;
      break;
    case 'github':
      logo = <ResourceIcon name={'github'} width={sizetoInnerPx(size)} />;
      break;
    case 'circleci':
      logo = <ResourceIcon name={'circleci'} width={sizetoInnerPx(size)} />;
      break;
    case 'kubernetes':
      logo = <ResourceIcon name={'kube'} width={sizetoInnerPx(size)} />;
      break;
    case 'azure':
      logo = <ResourceIcon name={'azure'} width={sizetoInnerPx(size)} />;
      break;
    case 'gitlab':
      logo = <ResourceIcon name={'gitlab'} width={sizetoInnerPx(size)} />;
      break;
    case 'gcp':
      logo = <ResourceIcon name={'googlecloud'} width={sizetoInnerPx(size)} />;
      break;
    case 'spacelift':
      logo = <ResourceIcon name={'spacelift'} width={sizetoInnerPx(size)} />;
      break;
    case 'terraform_cloud':
      logo = <ResourceIcon name={'terraform'} width={sizetoInnerPx(size)} />;
      break;
    case 'bitbucket':
      logo = <ResourceIcon name={'git'} width={sizetoInnerPx(size)} />;
      break;
    case 'oracle':
      // TODO(nicholasmarais1158): Add missing oracle icon/logo
      logo = <ResourceIcon name={'database'} width={sizetoInnerPx(size)} />;
      break;
    case 'azure_devops':
      logo = <ResourceIcon name={'azure'} width={sizetoInnerPx(size)} />;
      break;
  }

  if (logo) {
    return <ResourceIconContainer size={size}>{logo}</ResourceIconContainer>;
  }

  return null;
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

const ResourceIconContainer = styled(Flex)<{ size: IconProps['size'] }>`
  width: ${({ size }) => sizetoOuterPx(size)};
  height: ${({ size }) => sizetoOuterPx(size)};
  align-items: center;
  justify-content: center;
`;

function sizetoOuterPx(size: IconProps['size']) {
  if (size === 'small') return '16px';
  if (size === 'medium') return '20px';
  if (size === 'large') return '24px';
  if (size === 'extra-large') return '32px';
  return '24px';
}

function sizetoInnerPx(size: IconProps['size']) {
  if (size === 'small') return '12px';
  if (size === 'medium') return '16px';
  if (size === 'large') return '20px';
  if (size === 'extra-large') return '24px';
  return '24px';
}
