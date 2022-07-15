import React from 'react';
import { LicenseWarning } from './LicenseWarning';
import type { Props } from './LicenseWarning';

export default {
  title: 'TeleportE/Banner/LicenseWarning',
};

export const Error = () => {
  return <LicenseWarning {...props} />;
};

export const Info = () => {
  return <LicenseWarning {...props} severity="info" />;
};

export const Warning = () => {
  return <LicenseWarning {...props} severity="warning" />;
};

const props: Props = {
  severity: 'error',
  text: 'Your Teleport license has expired. If you are the System Administrator, please reach out to your Account Manager and obtain a new license to continue using Teleport.',
};
