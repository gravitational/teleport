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
    return api.delete(cfg.getContactsUrl(clusterId), {
      verify_token: verifyToken,
      contact_type: contactType,
    });
  },
};
