import React from 'react';
import { format } from 'date-fns';
import { Box, Text, Link, ButtonPrimary, Flex } from 'design';

import { Attempt } from 'shared/hooks/useAttemptNext';
import Alert from 'design/Alert';

import { GETTING_STARTED_LINK, LICENSE_FILE_GUIDE_LINK } from '../Downloads';

type LicenseProps = {
  canDownloadLicense: boolean;
  downloadLicense: () => void;
  licenseAttempt: Attempt;
  expiry?: Date;
};

export const License = ({
  canDownloadLicense,
  downloadLicense,
  licenseAttempt,
  expiry,
}: LicenseProps) => {
  return (
    <Box mt={6} mb={6}>
      <Text bold typography="h5">
        Download Your License Key
      </Text>
      {licenseAttempt.status === 'failed' && (
        <Alert kind="danger" children={licenseAttempt.statusText} mt={4} />
      )}
      <Text mt={3}>
        You will need to add your license file to authorize your deployment and
        update it anytime your contract updates. See{' '}
        <Link href={GETTING_STARTED_LINK} target="_blank" color="light">
          Getting Started with Teleport Enterprise
        </Link>{' '}
        and our{' '}
        <Link href={LICENSE_FILE_GUIDE_LINK} target="_blank" color="light">
          Enterprise License File Guide
        </Link>{' '}
        for more details on how this file is used.
      </Text>
      <Flex alignItems="center" mt={4}>
        <ButtonPrimary
          width="240px"
          mr={4}
          onClick={downloadLicense}
          size="large"
          disabled={
            licenseAttempt.status === 'processing' || !canDownloadLicense
          }
          title={
            canDownloadLicense
              ? 'Download license'
              : 'No permission to download license'
          }
        >
          {licenseAttempt.status === 'processing'
            ? 'Loading...'
            : 'Download License Key'}
        </ButtonPrimary>
        {expiry && (
          <Text color="grey.A100">
            Valid until {format(expiry, 'MM/dd/yyyy')}
          </Text>
        )}
      </Flex>
    </Box>
  );
};
