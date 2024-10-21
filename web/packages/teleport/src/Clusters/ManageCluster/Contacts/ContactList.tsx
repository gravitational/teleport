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

export type EmailListProps = {
  maxContacts: number;
  contacts: Contact[];
  onSubmit: (contact: Contact) => void;
  onDelete: (contact: Contact) => void;
  isLoading: boolean;
};

// TODO: email validation
// TODO: rename to contact list
export function ContactList({
  maxContacts,
  contacts,
  onSubmit,
  onDelete,
  isLoading,
}: EmailListProps) {
  // stores in the component's state newly
  //  created contact inputs that weren't commited to the backend yet
  const [newContacts, setNewContacts] = useState<Contact[]>([]);

  const qtyVerifiedContacts = contacts.filter(
    c => c.status === ContactStatus.VERIFIED
  ).length;
  const qtyContacts = contacts.length + newContacts.length;

  function handleDelete(contact: Contact, isNew: boolean) {
    if (isNew) {
      setNewContacts(prev => prev.filter(e => e.email !== contact.email));
      return;
    }
    // call onDelete
    onDelete(contact);
  }

  function handleNew() {
    if (qtyContacts >= maxContacts) {
      return;
    }

    setNewContacts(prev => [
      ...prev,
      {
        email: '',
        status: ContactStatus.PENDING,
        // Actual ID will be provided by the server when the user submits the contact,
        // for now we just need something that won't conflict with other new contacts.
        id: Math.floor(Math.random() * 100000).toString(),
        business: false,
        security: false,
      },
    ]);
  }

  function handleNewContactChange(id: string, email: string) {
    setNewContacts(prev => prev.map(e => (e.id !== id ? e : { ...e, email })));
  }

  function handleSubmit(contact: Contact) {
    onSubmit(contact);
    setNewContacts(prev => prev.filter(e => e.email !== contact.email));
    return;
  }

  return (
    <Flex flexDirection="column" width="100%" gap="3">
      <H3 mb="1">Email Address</H3>
      {/* existing contacts */}
      {contacts.map(contact => (
        <ContactInput
          key={contact.id}
          contact={contact}
          onChange={null} // existing contacts cannot be updated
          onSubmit={null} // existing contacts cannot be resubmitted
          onDelete={contact => handleDelete(contact, false)}
          isNew={false}
          disableInput={true}
          disableAction={
            isLoading ||
            (contact.status === ContactStatus.VERIFIED &&
              qtyVerifiedContacts <= 1)
          }
        />
      ))}
      {/* new contacts */}
      {newContacts.map(contact => (
        <ContactInput
          key={contact.id}
          contact={contact}
          onChange={handleNewContactChange}
          onSubmit={handleSubmit}
          onDelete={contact => handleDelete(contact, true)}
          disableInput={isLoading}
          disableAction={isLoading}
          isNew={true}
        />
      ))}
      <Box maxWidth="108px">
        <Button
          px="3"
          py="2"
          disabled={contacts.length >= maxContacts}
          onClick={handleNew}
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
  isNew: boolean;
  disableInput: boolean;
  disableAction: boolean;
};

function ContactInput({
  contact,
  onChange,
  onDelete,
  onSubmit,
  isNew,
  disableInput,
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
        disabled={disableInput}
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
