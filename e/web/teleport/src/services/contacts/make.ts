import { Contact } from './types';

export function makeContact(json: any): Contact {
  const {
    name,
    accountID,
    verifyToken,
    email,
    contactType,
    verified,
    verifyExpiresAt,
  } = json;
  return {
    name: name || '',
    accountId: accountID,
    verifyToken,
    email,
    contactType: contactType,
    verified: verified || false,
    verifyExpiresAt,
  };
}
