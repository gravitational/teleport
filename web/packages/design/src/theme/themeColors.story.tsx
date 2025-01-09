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

import Box, { BoxProps } from '../Box';
import Flex, { FlexProps } from '../Flex';
import Link from '../Link';
import Text, { H1, H2 } from '../Text';
import { Theme, ThemeColors } from '../theme';

export default {
  title: 'Design/Theme/Colors',
};

export const Colors = () => <ColorsComponent />;

const ColorsComponent = () => {
  const theme = useTheme();

  return (
    <Flex flexDirection="column" p="4">
      <Flex flexDirection="column">
        <H1 mb="2">Levels</H1>
        <Text mb="2">
          Levels are used to reflect the perceived depth of elements in the UI.
          The further back an element is, the more "sunken" it is, and the more
          forwards it is, the more "elevated" it is (think CSS z-index). <br />A
          "sunken" colour would be used to represent something like the
          background of the app. While "surface" would be the colour of the
          primary surface where most content is located (such as tables). Any
          colours more "elevated" than that would be used for things such as
          popovers, menus, and dialogs. <br />
          You can read more on this concept{' '}
          <Link
            target="_blank"
            href="https://m3.material.io/styles/elevation/applying-elevation"
          >
            here.
          </Link>
        </Text>
        <ColorsBox mb="4" colors={theme.colors.levels} themeType="levels" />
        <H1 mb="2">Interactive Colors</H1>
        <Text mb="2">
          <p>
            Interactive colors are used for hover states, indicators, etc. An
            example of this in use currently would be unified resource cards in
            the Pinned and Pinned (Hovered) states.
          </p>
          <p>
            All interactive colors have separate fields for background and text.
          </p>
        </Text>
        <Flex flexDirection="column" gap={4}>
          {Object.entries(theme.colors.interactive.solid).map(
            ([intent, colorGroup]) => (
              <Flex gap={4}>
                {Object.entries(colorGroup).map(([state, background]) => (
                  <SingleColorBox
                    mb="2"
                    path={`theme.colors.interactive.solid.${intent}.${state}`}
                    bg={background}
                    color={theme.colors.text.primaryInverse}
                  />
                ))}
              </Flex>
            )
          )}
        </Flex>
        <H2 mb="1">Tonal color variants</H2>
        <Text mb="2">
          Tonal color variants are used as highlights or accents. They are not
          solid colours, instead, they are a slightly transparent mask that
          highlights the colour behind it. This makes them quite versatile and
          they can be used to accentuate or highlight components on any
          background.
        </Text>
        <Flex flexDirection="column" gap={4}>
          {Object.entries(theme.colors.interactive.tonal).map(
            ([intent, colorGroup]) => (
              <Flex gap={4}>
                {colorGroup.map((background, i) => (
                  <SingleColorBox
                    mb="2"
                    path={`theme.colors.interactive.tonal.${intent}[${i}]`}
                    bg={background}
                    color={theme.colors.text.main}
                  />
                ))}
              </Flex>
            )
          )}
        </Flex>
        <H1 mb="2">Brand</H1>
        <SingleColorBox
          mb="4"
          path="theme.colors.brand"
          bg={theme.colors.brand}
          color={theme.colors.text.primaryInverse}
        />
        <H1 mb="2">Shadows</H1>
        <Flex>
          <Box
            mb={4}
            mr={6}
            css={`
              height: 200px;
              width: 200px;
              border-radius: 8px;
              display: flex;
              justify-content: center;
              align-items: center;
              text-align: center;
              background: ${theme.colors.levels.surface};
              box-shadow: ${theme.boxShadow[0]};
            `}
          >
            <Text>theme.boxShadow[0]</Text>
          </Box>
          <Box
            mb={4}
            mr={6}
            css={`
              height: 200px;
              width: 200px;
              border-radius: 8px;
              display: flex;
              justify-content: center;
              align-items: center;
              text-align: center;
              background: ${theme.colors.levels.surface};
              box-shadow: ${theme.boxShadow[1]};
            `}
          >
            <Text>theme.boxShadow[1]</Text>
          </Box>
          <Box
            mb={4}
            mr={6}
            css={`
              height: 200px;
              width: 200px;
              border-radius: 8px;
              display: flex;
              justify-content: center;
              align-items: center;
              text-align: center;
              background: ${theme.colors.levels.surface};
              box-shadow: ${theme.boxShadow[2]};
            `}
          >
            <Text>theme.boxShadow[2]</Text>
          </Box>
        </Flex>
        <H1 mb="2">Text Colors</H1>
        <Flex width="fit-content" flexDirection="row" mb={4}>
          <Flex
            flexDirection="column"
            border={`1px solid ${theme.colors.text.muted}`}
            bg={theme.colors.levels.surface}
            py={3}
            px={3}
            mr={3}
          >
            <Text>theme.colors.text.main</Text>
            <Text typography="h1" color={theme.colors.text.main}>
              Primary
            </Text>
            <Text typography="h2" color={theme.colors.text.main}>
              Primary
            </Text>
            <Text typography="h3" color={theme.colors.text.main}>
              Primary
            </Text>
            <Text typography="h4" color={theme.colors.text.main}>
              Primary
            </Text>
            <Text typography="body1" color={theme.colors.text.main}>
              Primary
            </Text>
          </Flex>
          <Flex
            flexDirection="column"
            border={`1px solid ${theme.colors.text.muted}`}
            bg={theme.colors.levels.surface}
            py={3}
            px={3}
            mr={3}
          >
            <Text>theme.colors.text.slightlyMuted</Text>
            <Text typography="h1" color={theme.colors.text.slightlyMuted}>
              Secondary
            </Text>
            <Text typography="h2" color={theme.colors.text.slightlyMuted}>
              Secondary
            </Text>
            <Text typography="h3" color={theme.colors.text.slightlyMuted}>
              Secondary
            </Text>
            <Text typography="h4" color={theme.colors.text.slightlyMuted}>
              Secondary
            </Text>
            <Text typography="body1" color={theme.colors.text.slightlyMuted}>
              Secondary
            </Text>
          </Flex>
          <Flex
            flexDirection="column"
            border={`1px solid ${theme.colors.text.muted}`}
            bg={theme.colors.levels.surface}
            py={3}
            px={3}
            mr={3}
          >
            <Text>theme.colors.text.muted</Text>
            <Text typography="h1" color={theme.colors.text.muted}>
              Placeholder
            </Text>
            <Text typography="h2" color={theme.colors.text.muted}>
              Placeholder
            </Text>
            <Text typography="h3" color={theme.colors.text.muted}>
              Placeholder
            </Text>
            <Text typography="h4" color={theme.colors.text.muted}>
              Placeholder
            </Text>
            <Text typography="body1" color={theme.colors.text.muted}>
              Placeholder
            </Text>
          </Flex>
          <Flex
            flexDirection="column"
            border={`1px solid ${theme.colors.text.muted}`}
            bg={theme.colors.levels.surface}
            py={3}
            px={3}
            mr={3}
          >
            <Text>theme.colors.text.disabled</Text>
            <Text typography="h1" color={theme.colors.text.disabled}>
              Disabled
            </Text>
            <Text typography="h2" color={theme.colors.text.disabled}>
              Disabled
            </Text>
            <Text typography="h3" color={theme.colors.text.disabled}>
              Disabled
            </Text>
            <Text typography="h4" color={theme.colors.text.disabled}>
              Disabled
            </Text>
            <Text typography="body1" color={theme.colors.text.disabled}>
              Disabled
            </Text>
          </Flex>
          <Flex
            flexDirection="column"
            border={`1px solid ${theme.colors.text.muted}`}
            bg={theme.colors.text.main}
            py={3}
            px={3}
            mr={3}
          >
            <Text color={theme.colors.text.primaryInverse}>
              theme.colors.text.primaryInverse
            </Text>
            <Text typography="h1" color={theme.colors.text.primaryInverse}>
              Primary Inverse
            </Text>
            <Text typography="h2" color={theme.colors.text.primaryInverse}>
              Primary Inverse
            </Text>
            <Text typography="h3" color={theme.colors.text.primaryInverse}>
              Primary Inverse
            </Text>
            <Text typography="h4" color={theme.colors.text.primaryInverse}>
              Primary Inverse
            </Text>
            <Text typography="body1" color={theme.colors.text.primaryInverse}>
              Primary Inverse
            </Text>
          </Flex>
        </Flex>
      </Flex>
    </Flex>
  );
};

function ColorsBox({
  colors,
  themeType = undefined,
  ...styles
}: {
  colors: ThemeColors['levels'];
  themeType?: string;
} & FlexProps) {
  const list = Object.entries(colors).map(([key, colorsForKey]) => {
    const fullPath = themeType
      ? `theme.colors.${themeType}.${key}`
      : `theme.colors.${key}`;

    return (
      <Flex flexWrap="wrap" key={key} width="260px" mb={3}>
        <Box
          css={`
            color: ${(props: { theme: Theme }) =>
              props.theme.colors.text.slightlyMuted};
          `}
        >
          {fullPath}
        </Box>
        <Box
          width="100%"
          height="50px"
          p={3}
          mr={3}
          css={`
            background: ${colorsForKey};
            border: 1px solid
              ${(props: { theme: Theme }) =>
                props.theme.colors.text.primaryInverse};
          `}
        />
      </Flex>
    );
  });

  return (
    <Flex flexWrap="wrap" {...styles}>
      {list}
    </Flex>
  );
}

function SingleColorBox({
  bg,
  color,
  path,
  ...styles
}: {
  bg: string;
  color: string;
  path: string;
} & BoxProps) {
  return (
    <Box width="150px" height="150px" p={3} mr={3} bg={bg} {...styles}>
      <Text color={color}>
        {/* Path, potentially broken along the periods. */}
        {path.split('.').map((word, i, arr) => (
          <>
            {word}
            {i < arr.length - 1 ? '.' : ''}
            {/*potential line break*/}
            <wbr />
          </>
        ))}
      </Text>
    </Box>
  );
}
