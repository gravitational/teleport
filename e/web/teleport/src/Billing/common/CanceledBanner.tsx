import { Box } from 'design';
import React from 'react';
import { useTheme } from 'styled-components';
import Link from 'design/Link';

export const CanceledBanner = () => {
  const theme = useTheme();

  // todo (michellescripts) add final payment date information as part of https://github.com/gravitational/cloud/issues/3536
  return (
    <Box
      bg={theme.colors.error.main}
      color={theme.colors.text.primaryInverse}
      m="0 -40px"
      p="20px 0 20px 40px"
    >
      <h2>Your account is canceled</h2>
      <i>
        Your account will be deleted once the grace period is over. To cancel
        this process, please&nbsp;
        <Link
          color={theme.colors.text.primaryInverse}
          href="https://goteleport.com/support/"
          target="_blank"
        >
          contact us.
        </Link>
      </i>
    </Box>
  );
};
