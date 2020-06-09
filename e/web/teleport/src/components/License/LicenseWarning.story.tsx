import React from 'react';
import LicenseWarning from './LicenseWarning';

export default {
  title: 'TeleportE/LicenseWarning',
};

export const Unpaid = () => {
  const html =
    `We were unable to process payment for your Teleport license subscription. ` +
    `Please update the payment information in your <a href="%v"> customer dashboard </a> ` +
    `or create a support  ticket at <a href="%v">  our support center </a> so that ` +
    `we can resolve the issue. Failure to resolve this issue is a violation of the ` +
    `<a href="%v"> Terms of Service </a> for Teleport.`;

  return <LicenseWarning html={html} onClose={() => null} />;
};

export const InvalidLicense = () => {
  const html =
    `Invalid Teleport license. Please create a support ` +
    `ticket at <a href="%v"> our support center </a> so that ` +
    `we can resolve the issue. Failure to resolve this issue is a violation of the ` +
    `<a href="%v"> Terms of Service </a> for Teleport.`;

  return <LicenseWarning html={html} onClose={() => null} />;
};
