import React from 'react';
import { Box, Link } from 'design';
import getDownloadLink from 'teleport/services/links';
import useTeleport from 'teleport/useTeleport';

export default function DownloadLinks({ version }: Props) {
  const ctx = useTeleport();
  const isEnterprise = ctx.isEnterprise;

  return (
    <Box>
      <Link
        href={getDownloadLink('mac', version, isEnterprise)}
        target="_blank"
        mr="2"
      >
        MacOS
      </Link>
      <Link
        href={getDownloadLink('linux64', version, isEnterprise)}
        target="_blank"
        mr="2"
      >
        Linux 64-bit
      </Link>
      <Link
        href={getDownloadLink('linux32', version, isEnterprise)}
        target="_blank"
      >
        Linux 32-bit
      </Link>
    </Box>
  );
}

type Props = {
  version: string;
};
