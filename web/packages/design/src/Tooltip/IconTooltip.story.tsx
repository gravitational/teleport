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

import styled, { useTheme } from 'styled-components';

import { ButtonPrimary, Flex, Text } from 'design';
import AGPLLogoDark from 'design/assets/images/agpl-dark.svg';
import AGPLLogoLight from 'design/assets/images/agpl-light.svg';
import { P } from 'design/Text/Text';

import { HoverTooltip } from './HoverTooltip';
import { IconTooltip } from './IconTooltip';

export default {
  title: 'Design/Tooltip',
};

export const ShortContent = () => (
  <Grid>
    <div style={{ gridColumn: '2/3' }}>
      <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
        Hover the icon
      </span>
      <IconTooltip position="bottom">"some popover content"</IconTooltip>
    </div>
    <div style={{ gridColumn: '1/2' }}>
      <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
        Hover the icon
      </span>
      <IconTooltip position="right">"some popover content"</IconTooltip>
    </div>
    <div style={{ gridColumn: '3/4' }}>
      <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
        Hover the icon
      </span>
      <IconTooltip position="left">"some popover content"</IconTooltip>
    </div>
    <div style={{ gridColumn: '2/3' }}>
      <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
        Hover the icon
      </span>
      <IconTooltip position="top">"some popover content"</IconTooltip>
    </div>
  </Grid>
);

const Grid = styled.div`
  display: grid;
  grid-template-columns: repeat(3, 200px);
  grid-template-rows: repeat(3, 100px);
`;

const logos = {
  light: AGPLLogoLight,
  dark: AGPLLogoDark,
};

export const LongContent = () => {
  const theme = useTheme();
  return (
    <>
      <Flex alignItems="center" mb={3}>
        <Text mr={1}>Hover the icon</Text>
        <IconTooltip>
          <P>
            Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do
            eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim
            ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut
            aliquip ex ea commodo consequat.
          </P>
          <P>
            Duis aute irure dolor in reprehenderit in voluptate velit esse
            cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat
            cupidatat non proident, sunt in culpa qui officia deserunt mollit
            anim id est laborum.
          </P>
        </IconTooltip>
      </Flex>
      <P>
        Here's some content that shouldn't interfere with the semi-transparent
        background:
      </P>
      <P>
        <div style={{ float: 'left', margin: '0 10px 10px 0' }}>
          <img src={logos[theme.type]} style={{ float: 'left' }} />
        </div>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
        veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
        commodo consequat.
      </P>
      <P>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non
        proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
      </P>
    </>
  );
};

export const WithMutedIconColor = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <IconTooltip muteIconColor>"some popover content"</IconTooltip>
  </>
);

export const WithKindWarning = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <IconTooltip kind="warning">"some popover content"</IconTooltip>
  </>
);

export const WithKindError = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <IconTooltip kind="error">"some popover content"</IconTooltip>
  </>
);

export const HoverToolTip = () => (
  <Flex alignItems="baseline" gap={2}>
    <span>Hover the</span>
    <HoverTooltip placement="bottom" tipContent="some popover content">
      <ButtonPrimary>button</ButtonPrimary>
    </HoverTooltip>
  </Flex>
);
