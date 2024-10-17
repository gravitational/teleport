import { Box, Flex, H2, Text } from 'design';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import { ContactEmail, EmailList } from './EmailList';
import styled from 'styled-components';
import * as Icons from 'design/Icon';
import { useState } from 'react';

export function BusinessContacts({}) {
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
    setEmails(prevEmails =>
      prevEmails.map(e => (e.id !== contactId ? e : { ...e, email }))
    );
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
            <H2 mb="2">Business Contacts</H2>
            <Text>
              Used for account and billing notices. Your Teleport account can
              have up-to 3 business contacts.
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

export const StyledIconBox = styled(Flex)`
  padding: ${props => props.theme.space[2]}px;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-radius: 8px;
  border: 1px solid ${props => props.theme.colors.interactive.tonal.neutral[2]};
`;

// TODO: reuse from ManageCluster
export const StyledRow = styled(Row)`
  @media screen and (max-width: ${props => props.theme.breakpoints.mobile}px) {
    border: none !important;
    padding-left: 0;
    padding-bottom: 0;
  }
`;
