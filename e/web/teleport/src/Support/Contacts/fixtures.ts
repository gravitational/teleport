import {
  Contact,
  ContactType,
  ContactVerification,
} from 'e-teleport/services/contacts/types';

export const contacts: Contact[] = [
  {
    name: '',
    accountId: 'account123',
    verifyToken: 'token123',
    email: 'alice@goteleport.com',
    contactType: ContactType.Business,
    verification: ContactVerification.Verified,
  },
  {
    name: '',
    accountId: 'account123',
    verifyToken: 'token456',
    email: 'bob@goteleport.com',
    contactType: ContactType.Security,
    verification: ContactVerification.Verified,
  },
  {
    name: '',
    accountId: 'account123',
    verifyToken: 'token789',
    email: 'business@goteleport.com',
    contactType: ContactType.Business,
    verification: ContactVerification.Verified,
  },
];
