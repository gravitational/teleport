import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import { makeContact } from './make';
import { Contact, ContactType } from './types';

export const contactsService = {
  fetchContacts(clusterId: string): Promise<Contact[]> {
    return api
      .get(cfg.getContactsUrl(clusterId))
      .then(res => res.contacts.map(makeContact));
  },

  createContact(
    clusterId: string,
    email: string,
    contactType: ContactType
  ): Promise<Contact> {
    return api
      .post(cfg.getContactsUrl(clusterId), { email, contactType })
      .then(res => makeContact(res));
  },

  deleteContact(
    clusterId: string,
    verifyToken: string,
    contactType: ContactType
  ): Promise<void> {
    return api.deleteWithOptions(cfg.getContactsUrl(clusterId), {
      data: {
        verify_token: verifyToken,
        contact_type: contactType,
      },
      // We have to explicitly set the `Content-Type` here otherwise it will default to text/plain.
      headers: { 'Content-Type': 'application/json' },
    });
  },
};
