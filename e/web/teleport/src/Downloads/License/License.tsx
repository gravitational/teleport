import React from 'react';
import Box from 'design/Box';
import Text from 'design/Text';
import Link from 'design/Link';
import { ButtonPrimary } from 'design/Button';
import { Attempt } from 'shared/hooks/useAttemptNext';
import Alert from 'design/Alert';

import { GETTING_STARTED_LINK, LICENSE_FILE_GUIDE_LINK } from '../Downloads';

type LicenseProps = {
  canDownloadLicense: boolean;
  downloadLicense: () => void;
  licenseAttempt: Attempt;
};

export const License = ({
  canDownloadLicense,
  downloadLicense,
  licenseAttempt,
}: LicenseProps) => {
  return (
    <>
      <Box mt={6} mb={6}>
        <Text bold typography="h5">
          Download Your License Key
        </Text>
        <Text mt={3}>
          You will need to add your license file to authorize your deployment
          and update it anytime your contract updates. See{' '}
          <Link href={GETTING_STARTED_LINK} target="_blank" color="light">
            Getting Started with Teleport Enterprise
          </Link>{' '}
          and our{' '}
          <Link href={LICENSE_FILE_GUIDE_LINK} target="_blank" color="light">
            Enterprise License File Guide
          </Link>{' '}
          for more details on how this file is used.
        </Text>
        <ButtonPrimary
          mt={4}
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
          Download License Key
        </ButtonPrimary>
      </Box>
      {licenseAttempt.status === 'failed' && (
        <Alert kind="danger" children={licenseAttempt.statusText} />
      )}
    </>
  );
};
