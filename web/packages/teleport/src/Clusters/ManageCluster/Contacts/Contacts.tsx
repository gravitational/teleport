import React, { useEffect, useState } from 'react';

import Box from 'design/Box';

import useAttempt from 'shared/hooks/useAttemptNext';

import { useTeleport } from 'teleport/index';
import { contactsService } from 'teleport/services/contacts';
import { Contact } from 'teleport/services/contacts/contacts';

import { BusinessContacts } from './BusinessContacts';
import { SecurityContacts } from './SecurityContacts';

const MAX_CONTACTS = 3;
type ContactType = 'business' | 'security';

// TODO:check permissions
export function Contacts() {
  const [contacts, setContacts] = useState<Contact[]>([]);
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;
  const { attempt, run } = useAttempt();

  console.log('contacts', contacts);
  const businessContacts = contacts.filter(c => c.business);
  console.log('businessContacts', businessContacts);
  const securityContacts = contacts.filter(c => c.security);

  useEffect(() => {
    async function init() {
      const res = await contactsService.getContacts(cluster.clusterId);
      setContacts(res);
    }

    run(init);
  }, [cluster.clusterId, run]);

  function handleSubmit(contact: Contact, contactType: ContactType) {
    const security = contactType === 'security';
    const business = !security;
    run(() =>
      contactsService
        .createContact(cluster.clusterId, {
          email: contact.email,
          security,
          business,
        })
        .then(resp => {
          console.log('got back', resp);
          // resp may or may not be a new contact, since the server will reuse
          // an existing contact if its email already exists among the cluster's contacts
          setContacts(prev => prev.map(e => (e.id === resp.id ? resp : e)));
        })
    );
  }

  const isLoading = attempt.status == 'processing';

  function handleDelete(contact: Contact, contactType: ContactType) {
    // toggle flag of the contact type we're updating
    // and use the previous value for the other one
    const security = contactType === 'security' ? false : contact.security;
    const business = contactType === 'business' ? false : contact.business;
    run(() =>
      contactsService
        .updateContact(cluster.clusterId, contact.id, {
          security,
          business,
        })
        .then(updatedContact => {
          console.log('got back', updatedContact);
          const newContacts = contacts.map(e =>
            e.id === updatedContact.id ? updatedContact : e
          );
          console.log('newContacts', newContacts);
          setContacts(newContacts);
        })
    );
  }

  // TODO warning component
  if (attempt.status === 'failed') {
    <Box mb="3">Erorr loading contacts: {attempt.statusText}</Box>;
  }

  // TODO: loading state

  return (
    <>
      <Box mb="3">
        <BusinessContacts
          contacts={businessContacts}
          maxContacts={MAX_CONTACTS}
          onSubmit={contact => handleSubmit(contact, 'business')}
          onDelete={contact => handleDelete(contact, 'business')}
          isLoading={isLoading}
        />
      </Box>
      <SecurityContacts
        contacts={securityContacts}
        maxContacts={MAX_CONTACTS}
        onSubmit={contact => handleSubmit(contact, 'security')}
        onDelete={contact => handleDelete(contact, 'security')}
        isLoading={isLoading}
      />
    </>
  );
}
