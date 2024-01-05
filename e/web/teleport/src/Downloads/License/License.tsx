import React from 'react';
import { format } from 'date-fns';
import { Box, Text, Link, ButtonPrimary, ButtonSecondary, Flex } from 'design';

import { Attempt } from 'shared/hooks/useAttemptNext';
import Alert from 'design/Alert';

import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';

import { GETTING_STARTED_LINK, LICENSE_FILE_GUIDE_LINK } from '../Downloads';

type LicenseProps = {
  canGenerateLicense: boolean;
  showSaveLicenseDialog: boolean;
  closeSaveLicenseDialog: () => void;
  generateLicense: () => void;
  saveLicense: () => void;
  licenseAttempt: Attempt;
  expiry?: Date;
};

export const License = ({
  canGenerateLicense,
  showSaveLicenseDialog,
  closeSaveLicenseDialog,
  generateLicense,
  saveLicense,
  licenseAttempt,
  expiry,
}: LicenseProps) => {
  return (
    <Box mt={6} mb={6}>
      <Text bold typography="h5">
        Generate Your License Key
      </Text>
      {licenseAttempt.status === 'failed' && (
        <Alert kind="danger" children={licenseAttempt.statusText} mt={4} />
      )}
      <Text mt={3}>
        You will need to add your license file to authorize your deployment and
        update it anytime your contract updates. See{' '}
        <Link href={GETTING_STARTED_LINK} target="_blank" color="text.main">
          Getting Started with Teleport Enterprise
        </Link>{' '}
        and our{' '}
        <Link href={LICENSE_FILE_GUIDE_LINK} target="_blank" color="text.main">
          Enterprise License File Guide
        </Link>{' '}
        for more details on how this file is used.
      </Text>
      <Flex alignItems="center" mt={4}>
        <ButtonPrimary
          width="240px"
          mr={4}
          onClick={generateLicense}
          size="large"
          disabled={
            licenseAttempt.status === 'processing' || !canGenerateLicense
          }
          title="Generate license"
        >
          {licenseAttempt.status === 'processing'
            ? 'Generating...'
            : 'Generate License Key'}
        </ButtonPrimary>
        {expiry && (
          <Text color="text.slightlyMuted">
            Valid until {format(expiry, 'MM/dd/yyyy')}
          </Text>
        )}
      </Flex>

      <DialogConfirmation
        dialogCss={() => ({ maxWidth: '50vw' })}
        open={showSaveLicenseDialog}
        onClose={closeSaveLicenseDialog}
      >
        <DialogHeader>
          <DialogTitle>New License Key Generated</DialogTitle>
        </DialogHeader>
        <DialogContent>
          Teleport has generated a new, unique license key. If you have multiple
          deployments, you must use the same license file in each one.
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary
            mr={3}
            onClick={() => {
              saveLicense();
              closeSaveLicenseDialog();
            }}
          >
            Download
          </ButtonPrimary>
          <ButtonSecondary onClick={closeSaveLicenseDialog}>
            Close
          </ButtonSecondary>
        </DialogFooter>
      </DialogConfirmation>
    </Box>
  );
};
