import { useEffect, useState } from 'react';

import Logger from 'shared/libs/logger';

import licenseService, { LicenseStatus } from 'e-teleport/services/license';

const logger = Logger.create('LicenseEnforcer');

export function useBanner() {
  const [license, setLicense] = useState<LicenseStatus>();

  useEffect(() => {
    licenseService
      .fetchStatus()
      .then(res => {
        if (!res) {
          return;
        }
        setLicense(res);
      })
      .catch(err => {
        logger.error(err);
      });
  }, []);

  return {
    license,
  };
}
