import React from 'react';
import { useTheme } from 'styled-components';
import { Box, Card, Flex, Text, Link } from 'design';
import { DevicesIcon } from 'design/SVGIcon';

export const EmptyList = () => {
  const theme = useTheme();
  return (
    <Card maxWidth="700px" p={4} as={Flex} alignItems="center">
      <Box style={{ textAlign: 'center' }} mr={5}>
        <DevicesIcon size={150} fill={theme.colors.spotBackground[2]} />
      </Box>

      <Box>
        <Text typography="h6" mb={3} caps>
          Register Trusted Device
        </Text>
        <Text typography="subtitle1" mb={3}>
          Device Trust allows Teleport admins to enforce the use of trusted
          devices. Resources protected by the device mode "required" will
          enforce the use of a trusted device, in addition to establishing the
          user's identity and enforcing the necessary roles.
        </Text>
        <Text typography="subtitle1">
          Furthermore, users using a trusted device leave audit trails that
          include the device's information. Please{' '}
          <Link
            color="text.primary"
            href="https://goteleport.com/docs/access-controls/guides/device-trust/"
            target="_blank"
          >
            view our documentation
          </Link>{' '}
          on how to get started with Device Trust.
        </Text>
      </Box>
    </Card>
  );
};
