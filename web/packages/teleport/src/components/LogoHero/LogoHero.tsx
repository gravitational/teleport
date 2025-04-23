/*
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

import { useTheme } from 'styled-components';

import AGPLLogoDark from 'design/assets/images/agpl-dark.svg';
import AGPLLogoLight from 'design/assets/images/agpl-light.svg';
import CommunityLogoDark from 'design/assets/images/community-dark.svg';
import CommunityLogoLight from 'design/assets/images/community-light.svg';
import EnterpriseLogoDark from 'design/assets/images/enterprise-dark.svg';
import EnterpriseLogoLight from 'design/assets/images/enterprise-light.svg';
import Image from 'design/Image';

import cfg, { TeleportEdition } from 'teleport/config';

type LogoMap = {
  light: string;
  dark: string;
};

export const logos: Record<TeleportEdition, LogoMap> = {
  oss: {
    light: AGPLLogoLight,
    dark: AGPLLogoDark,
  },
  community: {
    light: CommunityLogoLight,
    dark: CommunityLogoDark,
  },
  ent: {
    light: EnterpriseLogoLight,
    dark: EnterpriseLogoDark,
  },
};

export const LogoHero = ({ my = '48px' }: { my?: string }) => {
  const theme = useTheme();
  const src = logos[cfg.edition][theme.type];
  return (
    <Image src={src} maxHeight="120px" maxWidth="200px" my={my} mx="auto" />
  );
};
