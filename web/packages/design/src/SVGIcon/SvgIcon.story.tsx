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

import { Fragment, ReactNode } from 'react';
import { useTheme } from 'styled-components';

import { IconCircle } from 'design/Icon/IconCircle';

import Flex from '../Flex';
import Text from '../Text';
import { SVGIconProps } from './common';
import { TeleportGearIcon } from './TeleportGearIcon';

export default {
  title: 'Design/Icon',
};

export const CustomIcons = () => {
  const icons = Object.entries({ TeleportGearIcon });
  return (
    <Flex flexWrap="wrap">
      {icons.map(([icon, S]) => {
        const size = 64;

        return (
          <Fragment key={icon}>
            <IconBox text={icon}>
              <IconContainer Icon={S} size={size} />
            </IconBox>
            <IconBox text={icon}>
              <IconCircle Icon={S} size={size} />
            </IconBox>
          </Fragment>
        );
      })}
    </Flex>
  );
};

const IconBox = ({ children, text }: { children: ReactNode; text: string }) => {
  const theme = useTheme();

  return (
    <Flex
      width="150px"
      height="150px"
      alignItems="center"
      justifyContent="center"
      bg={theme.colors.spotBackground[0]}
      flexDirection="column"
      m={1}
    >
      <Flex justifyContent="center" p={2}>
        {children}
      </Flex>
      <Text typography="body3" mt={2}>
        {text}
      </Text>
    </Flex>
  );
};

const IconContainer = ({
  Icon,
  size,
}: {
  Icon: React.ComponentType<SVGIconProps>;
  size: number;
}) => {
  const theme = useTheme();

  return <Icon size={size} fill={theme.colors.text.main} />;
};
