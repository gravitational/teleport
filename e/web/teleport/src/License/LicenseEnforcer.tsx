/* eslint react/no-danger: 0 */

import React from 'react';
import Logger from 'shared/libs/logger';
import LicenseWarning from './LicenseWarning';
import licenseService, { LicenseStatus } from 'e-teleport/services/license';

const logger = Logger.create('LicenseEnforcer');

export default function LicenseEnforcer() {
  const [hasShown, setHasShown] = React.useState(false);
  const [license, setLicense] = React.useState<LicenseStatus>(null);

  React.useEffect(() => {
    licenseService
      .fetchStatus()
      .then(setLicense)
      .catch(err => {
        logger.error(err);
      });
  }, []);

  function onClose() {
    setHasShown(true);
  }

  if (!hasShown && Boolean(license)) {
    return <LicenseWarning html={license.html} onClose={onClose} />;
  }

  return null;
}
