import type { License } from './types';

export const makeLicense = (json: any): License => {
  const expiry = json?.expiry ? new Date(json.expiry) : undefined;
  return {
    pem: json?.pem || '',
    expiry,
  };
};
