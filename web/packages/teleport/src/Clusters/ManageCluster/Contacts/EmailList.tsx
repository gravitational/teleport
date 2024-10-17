import Box from 'design/Box';
import { Button } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { Plus, Trash, EmailSolid } from 'design/Icon';
import Input from 'design/Input';
import { Pill } from 'design/Pill';
import Text, { H2, H3 } from 'design/Text';
import { useState } from 'react';
import styled from 'styled-components';

export type ContactEmail = {
  email: string;
  status: 'verified' | 'pending' | 'expired' | 'uncommited';
  id: string;
};

export type EmailListProps = {
  maxEmails: number;
  emails: ContactEmail[];
  onContactSubmit: (contact: ContactEmail) => void;
  onContactDelete: (contact: ContactEmail) => void;
  onContactChange: (contactId: string, email: string) => void;
  onNewContact: () => void;
};

// TODO: email validation
// TODO: rename to contact list
export function EmailList({
  maxEmails,
  emails,
  onContactSubmit,
  onContactDelete,
  onNewContact,
  onContactChange,
}: EmailListProps) {
  const qtyVerifiedEmail = emails.filter(c => c.status === 'verified').length;
  console.log('Verified: ', qtyVerifiedEmail);

  return (
    <Flex flexDirection="column" width="100%" gap="3">
      <H3 mb="1">Email Address</H3>
      {emails.map(contact => (
        <Flex alignItems="center">
          <Input
            width="100%"
            icon={EmailSolid}
            placeholder="mail@example.com"
            value={contact.email}
            disabled={contact.status === 'verified' && qtyVerifiedEmail <= 1}
            onChange={e => onContactChange(contact.id, e.target.value)}
          />
          <Pill label={contact.status} />
          {/* TODO: status */}
          <StyledDeleteButton
            intent="neutral"
            size="small"
            ml="2"
            disabled={contact.status === 'verified' && qtyVerifiedEmail <= 1}
            onClick={() => onContactDelete(contact)}
          >
            <Trash size="small" />
            {/* TODO: chang icon to "send" if it is not set yet */}
          </StyledDeleteButton>
        </Flex>
      ))}
      {/* </Flex> */}
      <Box maxWidth="108px">
        <Button
          px="3"
          py="2"
          disabled={emails.length >= maxEmails}
          onClick={() => onNewContact()}
        >
          <Plus size="small" /> Add New
        </Button>
      </Box>
    </Flex>
  );
}

const StyledDeleteButton = styled(Button)`
  width: 40px;
  height: 40px;
`;
