import { useState } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import { Button } from 'design/Button';
import Flex from 'design/Flex';
import { Plus, Trash, EmailSolid, PaperPlane } from 'design/Icon';
import Input from 'design/Input';
import Label from 'design/Label';
import { LabelKind } from 'design/LabelState/LabelState';
import { H3 } from 'design/Text';

import { Contact, ContactStatus } from 'teleport/services/contacts/contacts';

export type ContactListProps = {
  maxContacts: number;
  contacts: Contact[];
  onSubmit: (contact: Contact) => void;
  onDelete: (contact: Contact) => void;
  onNew: () => void;
  onChange: (contactId: string, email: string) => void;
  isLoading: boolean;
};

// TODO: email validation
// TODO: rename to contact list
export function ContactList({
  maxContacts,
  contacts,
  onSubmit,
  onDelete,
  onNew,
  onChange,
  isLoading,
}: ContactListProps) {
  const qtyVerifiedContacts = contacts.filter(
    c => c.status === ContactStatus.VERIFIED
  ).length;

  return (
    <Flex flexDirection="column" width="100%" gap="3">
      <H3 mb="1">Email Address</H3>
      {contacts.map(contact => (
        <ContactInput
          key={contact.id}
          contact={contact}
          onChange={onChange}
          onSubmit={onSubmit}
          onDelete={onDelete}
          isNew={contact.status === ContactStatus.UNCOMMITED}
          isLoading={isLoading}
          disableAction={
            isLoading ||
            (contact.status === ContactStatus.VERIFIED &&
              qtyVerifiedContacts <= 1)
          }
        />
      ))}
      <Box maxWidth="108px">
        <Button
          px="3"
          py="2"
          disabled={contacts.length >= maxContacts}
          onClick={onNew}
        >
          <Plus size="small" /> Add New
        </Button>
      </Box>
    </Flex>
  );
}

type ContactInputProps = {
  contact: Contact;
  onChange?: (contactId: string, email: string) => void;
  onSubmit?: (contact: Contact) => void;
  onDelete: (contact: Contact) => void;
  isLoading: boolean;
  isNew: boolean;
  disableAction: boolean;
};

function ContactInput({
  contact,
  onChange,
  onDelete,
  onSubmit,
  isLoading,
  isNew,
  disableAction,
}: ContactInputProps) {
  return (
    <Flex alignItems="center">
      <Input
        width="100%"
        icon={EmailSolid}
        placeholder="mail@example.com"
        value={contact.email}
        onChange={e => onChange(contact.id, e.target.value)}
        disabled={isLoading || !isNew}
      />
      {!isNew && (
        <Label kind={getLabelKind(contact.status)}>
          {getStatusText(contact.status)}
        </Label>
      )}

      <StyledActionButton
        intent="neutral"
        size="small"
        ml="2"
        disabled={disableAction}
        onClick={() => (isNew ? onSubmit(contact) : onDelete(contact))}
      >
        {isNew ? <PaperPlane size="small" /> : <Trash size="small" />}

        {/* TODO: chang icon to "send" if it is not sent yet, update method */}
      </StyledActionButton>
    </Flex>
  );
}

const StyledActionButton = styled(Button)`
  width: 40px;
  height: 40px;
`;

function getLabelKind(status: ContactStatus): LabelKind {
  switch (status) {
    case ContactStatus.EXPIRED:
      return 'danger';
    case ContactStatus.PENDING:
      return 'warning';
    case ContactStatus.VERIFIED:
      return 'success';
  }
}

function getStatusText(status: ContactStatus): string {
  switch (status) {
    case ContactStatus.EXPIRED:
      return 'expired';
    case ContactStatus.PENDING:
      return 'pending';
    case ContactStatus.VERIFIED:
      return 'verified';
  }
}
