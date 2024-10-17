import { Box, Flex, H2, Text } from 'design';
import { MultiRowBox } from 'design/MultiRowBox';
import { ContactEmail, EmailList } from './EmailList';
import { StyledIconBox, StyledRow } from './BusinessContacts';
import * as Icons from 'design/Icon';
import { useState } from 'react';

export function SecurityContacts({}) {
  // TODO this will come as params or from context
  const [emails, setEmails] = useState<ContactEmail[]>([
    { email: 'matheus.battirola@goteleport.com', status: 'verified', id: '1' },
  ]);

  function handleDelete(contact: ContactEmail) {
    if (contact.status === 'uncommited') {
      setEmails(emails.filter(e => e.email !== contact.email));
    }
    // other statuses will send a request to the backend, then remove the email
    // in case of a success response
    // TODO call backend
    setEmails(emails.filter(e => e.id !== contact.id));
  }

  function handleContactChange(contactId: string, email: string) {
    setEmails(emails.map(e => (e.id !== contactId ? e : { ...e, email })));
  }

  function handleNew() {
    if (emails.length >= 3) {
      return;
    }

    setEmails([
      ...emails,
      {
        email: '',
        status: 'uncommited',
        id: Math.floor(Math.random() * 100000).toString(),
      },
    ]);
  }

  return (
    <MultiRowBox>
      <StyledRow>
        <Flex gap="3">
          <Flex
            flexDirection="column"
            width="100%"
            maxWidth="430px"
            alignItems="start"
            gap="2"
          >
            <StyledIconBox>
              <Icons.Lock size="medium" />
            </StyledIconBox>
            <H2 mb="2">Security Contacts</H2>
            <Text>
              Used for notices of important security patches and
              vulnerabilities. Your Teleport account can have up-to 3 security
              contacts.
            </Text>
          </Flex>
          <EmailList
            emails={emails}
            maxEmails={3}
            onContactDelete={handleDelete}
            onNewContact={handleNew}
            onContactSubmit={console.log}
            onContactChange={handleContactChange}
          />
        </Flex>
      </StyledRow>
    </MultiRowBox>
  );
}
