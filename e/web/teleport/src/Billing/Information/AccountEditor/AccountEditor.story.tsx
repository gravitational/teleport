import React from 'react';
import AccountEditor from './AccountEditor';
import { account } from '../../fixtures';

export default {
  title: 'TeleportE/Billing/AccountEditor',
};

export const Processing = () => {
  return <AccountEditor {...sample} attempt={{ status: 'processing' }} />;
};

export const Loaded = () => {
  return <AccountEditor {...sample} />;
};

export const LoadedEmpty = () => {
  return (
    <AccountEditor
      {...sample}
      account={{
        contactEmail: '',
        contactName: '',
        companyName: '',
        companyAddressCity: '',
        companyAddressCountry: '',
        companyAddressLine1: '',
        companyAddressLine2: '',
        companyAddressPostalCode: '',
        companyAddressState: '',
        balance: 33,
      }}
    />
  );
};

export const Failed = () => {
  return (
    <AccountEditor
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const sample = {
  attempt: {
    status: 'success' as any,
  },
  account,
  onClose: () => null,
  onSave: () => null,
};
