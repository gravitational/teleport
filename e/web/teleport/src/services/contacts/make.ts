import { Contact, ContactVerification } from './types';

export function makeContact(json: any): Contact {
  const { name, accountID, verifyToken, email, contactType, contactState } =
    json;
  return {
    name: name || '',
    accountId: accountID,
    verifyToken,
    email,
    contactType,
    verification: makeVerification(contactState),
  };
}

function makeVerification(contactState: string): ContactVerification {
  // values match the Cloud tenant API's ENUM
  switch (contactState) {
    case 'CONTACT_STATE_ACTIVE':
      return ContactVerification.Verified;
    case 'CONTACT_STATE_PENDING':
      return ContactVerification.Pending;
    case 'CONTACT_STATE_EXPIRED':
      return ContactVerification.Expired;
  }
  // Return Pending as a default, but this case
  // should never be reached
  return ContactVerification.Pending;
}
