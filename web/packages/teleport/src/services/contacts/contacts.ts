import cfg from 'teleport/config';
import api from 'teleport/services/api';

// Should match `e/lib/web/ui/contact.go`, except for UNCOMMITED
export enum ContactStatus {
  PENDING = 0,
  VERIFIED = 1,
  EXPIRED = 2,
  // UI-only status, signals that a contact exists only on the web UI state
  UNCOMMITED = 1000,
}

export type Contact = {
  id: string;
  email: string;
  status: ContactStatus;
  business: boolean;
  security: boolean;
};

export type CreateContactRequest = {
  email: string;
  business: boolean;
  security: boolean;
};

export type UpdateContactRequest = {
  business: boolean;
  security: boolean;
};

export const contactsService = {
  getContacts(clusterId: string): Promise<Contact[]> {
    return api
      .get(cfg.getContactUrl(clusterId))
      .then(resp => resp?.map(makeContact));
  },

  createContact(clusterId: string, req: CreateContactRequest): Promise<Contact> {
    return api.post(cfg.getContactUrl(clusterId), req).then(makeContact);
  },

  updateContact(
    clusterId: string,
    contactId: string,
    req: UpdateContactRequest
  ): Promise<Contact> {
    return api.patch(`${cfg.getContactUrl(clusterId)}/${contactId}`, req).then(makeContact);
  },
};

// TODO tests
export function makeContact(json): Contact {
  const { id, email, status, business, security } = json;
  return {
    id,
    email,
    status,
    business,
    security,
  };
}
