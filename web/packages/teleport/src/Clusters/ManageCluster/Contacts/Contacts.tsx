import React, { useEffect, useState } from 'react';

import Box from 'design/Box';

import useAttempt from 'shared/hooks/useAttemptNext';

import { useTeleport } from 'teleport/index';
import { contactsService } from 'teleport/services/contacts';
import { Contact, ContactStatus } from 'teleport/services/contacts/contacts';

import { BusinessContacts } from './BusinessContacts';
import { SecurityContacts } from './SecurityContacts';

const MAX_CONTACTS = 3;
type ContactType = 'business' | 'security';

// TODO:check permissions
export function Contacts() {
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  const [contacts, setContacts] = useState<Contact[]>([]);

  const { attempt, run } = useAttempt();
  const isLoading = attempt.status == 'processing';

  const businessContacts = contacts.filter(c => c.business);
  const securityContacts = contacts.filter(c => c.security);

  console.log('contacts', contacts);
  console.log('business', businessContacts);
  console.log('security', securityContacts);

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
    const prevId = contact.id;
    run(() =>
      contactsService
        .createContact(cluster.clusterId, {
          email: contact.email,
          security,
          business,
        })
        .then(resp => {
          setContacts(
            prev =>
              prev
                .filter(c => c.id !== resp.id) // remove possible duplicates
                .map(c => (c.id === prevId ? { ...resp } : c)) // update the new contact with the server response
          );
          return;
        })
    );
  }

  function handleDelete(contact: Contact, contactType: ContactType) {
    console.log('delete contact', contact.id);
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
        .then(resp => {
          console.log('delete resp', resp);
          setContacts(prev =>
            prev.map(c => (c.id === resp.id ? { ...resp } : c))
          );
        })
    );
  }

  function handleNew(contactType: ContactType) {
    setContacts([
      ...contacts,
      {
        email: '',
        status: ContactStatus.UNCOMMITED,
        // Actual ID will be provided by the server when the user submits the contact,
        // for now we just need something that won't conflict with other new contacts.
        id: Math.floor(Math.random() * 100000).toString(),
        business: contactType === 'business',
        security: contactType === 'security',
      },
    ]);
  }

  function handleChange(contactId: string, email: string) {
    setContacts(prev =>
      prev.map(c =>
        c.id === contactId && c.status === ContactStatus.UNCOMMITED // only uncommited emails can be changed
          ? { ...c, email }
          : c
      )
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
          onNew={() => handleNew('business')}
          onChange={handleChange}
          isLoading={isLoading}
        />
      </Box>
      <SecurityContacts
        contacts={securityContacts}
        maxContacts={MAX_CONTACTS}
        onSubmit={contact => handleSubmit(contact, 'security')}
        onDelete={contact => handleDelete(contact, 'security')}
        onNew={() => handleNew('security')}
        onChange={handleChange}
        isLoading={isLoading}
      />
    </>
  );
}
