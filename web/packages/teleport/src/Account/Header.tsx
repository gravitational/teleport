import { Box, ButtonSecondary, Flex, Text } from 'design';
import React from 'react';
import styled, { useTheme } from 'styled-components';

export interface HeaderProps {
  title: string;
  description?: string;
  icon: React.ReactNode;
  actions: React.ReactNode;
}

export function Header({ title, description, icon, actions }: HeaderProps) {
  const theme = useTheme();
  return (
    <Flex alignItems="start" gap={3}>
      {/* lineHeight=0 prevents the icon background from being larger than
              required by the icon itself. */}
      <Box
        bg={theme.colors.interactive.tonal.neutral[0]}
        lineHeight={0}
        p={2}
        borderRadius={3}
      >
        {icon}
      </Box>
      <Box flex="1">
        <Text typography="h4">{title}</Text>
        <Text typography="body1" color={theme.colors.text.slightlyMuted}>
          {description}
        </Text>
      </Box>
      <Box flex="0 0 auto">{actions}</Box>
    </Flex>
  );
}

export const ActionButton = styled(ButtonSecondary)`
  padding: ${props => `${props.theme.space[2]}px ${props.theme.space[4]}px`};
  gap: ${props => `${props.theme.space[2]}px`};
  text-transform: none;
`;
