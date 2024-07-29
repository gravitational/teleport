import React from 'react';
import { useTheme } from 'styled-components';
import { Box, Card, Flex, Link, H3 } from 'design';
import { Laptop } from 'design/Icon';
import { P } from 'design/Text/Text';

export const EmptyList = () => {
  const theme = useTheme();
  return (
    <Card maxWidth="700px" p={4} as={Flex} alignItems="center">
      <Box style={{ textAlign: 'center' }} mr={5}>
        <Laptop size={150} color={theme.colors.spotBackground[2]} />
      </Box>

      <Box>
        <H3 mb={3}>Register Trusted Device</H3>
        <P>
          Device Trust enables authenticated device access. Resources protected
          by the Device Trust mode "required" will enforce the use of a Trusted
          Device, in addition to establishing the user's identity and enforcing
          the necessary roles.
        </P>
        <P>
          Trusted Devices can be registered manually using{' '}
          <Link
            color="text.main"
            href="https://goteleport.com/docs/access-controls/device-trust/guide/?scope=enterprise#step-12-register-a-trusted-device"
            target="_blank"
          >
            tctl client
          </Link>{' '}
          or synced automatically from MDM services like{' '}
          <Link
            color="text.main"
            href="https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise"
            target="_blank"
          >
            Jamf
          </Link>
          .
        </P>
        <P>
          Please{' '}
          <Link
            color="text.main"
            href="https://goteleport.com/docs/access-controls/guides/device-trust/"
            target="_blank"
          >
            view our documentation
          </Link>{' '}
          on how to get started with Device Trust.
        </P>
      </Box>
    </Card>
  );
};
