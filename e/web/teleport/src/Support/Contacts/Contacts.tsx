import {
  ButtonSecondary,
  ButtonWarning,
  ComposedAlert,
  ComposedDialog,
  Dialog,
  Subtitle2,
} from '@gravitational/design-system';
import { useCallback, useEffect, useState, type JSX } from 'react';

import * as Icons from 'design/Icon';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import {
  Contact,
  ContactType,
  ContactVerification,
} from 'e-teleport/services/contacts/types';
import useTeleportE from 'e-teleport/useTeleportE';
import { SupportSectionCard } from 'teleport/Support/Support';

import { ContactEmails, FormContact } from './ContactEmails';

const MAX_CONTACTS = 3;

export function Contacts({ clusterId }: { clusterId: string }) {
  const ctx = useTeleportE();

  const access = ctx.storeUser.getContactsAccess();

  const [businessContacts, setBusinessContacts] = useState<FormContact[]>([]);
  const [securityContacts, setSecurityContacts] = useState<FormContact[]>([]);

  const [listAttempt, listRun] = useAsync(
    useCallback(async () => {
      if (!access.list) {
        return;
      }
      const res = await ctx.contactService.fetchContacts(clusterId);
      const contacts: FormContact[] = res.map(c => ({
        ...c,
        draft: false,
        recentlyInvited: false,
      }));
      setBusinessContacts(
        contacts.filter(c => c.contactType & ContactType.Business)
      );
      setSecurityContacts(
        contacts.filter(c => c.contactType & ContactType.Security)
      );
      return res;
    }, [access, clusterId, ctx.contactService])
  );

  const runDelete = (verifyToken: string, type: ContactType) =>
    ctx.contactService.deleteContact(clusterId, verifyToken, type);
  const [businessDeleteAttempt, businessDeleteRun] = useAsync(runDelete);
  const [securityDeleteAttempt, securityDeleteRun] = useAsync(runDelete);

  const runCreate = (email: string, type: ContactType) =>
    ctx.contactService.createContact(clusterId, email, type);

  // create separate attempts for business and security contacts,
  // so errors can be shown invididually
  const [businessCreateAttempt, businessCreateRun] = useAsync(runCreate);
  const [securityCreateAttempt, securityCreateRun] = useAsync(runCreate);

  const [deletingTokenId, setDeletingTokenId] = useState<{
    verifyToken: string;
    type: ContactType;
  }>(null);

  useEffect(() => {
    if (!listAttempt.status && clusterId) {
      listRun();
    }
  }, [listAttempt.status, listRun, clusterId]);

  function handleChange(verifyToken: string, val: string, type: ContactType) {
    const setContacts =
      type === ContactType.Business ? setBusinessContacts : setSecurityContacts;

    setContacts(contacts =>
      contacts.map(c =>
        c.verifyToken === verifyToken ? { ...c, email: val } : c
      )
    );
  }

  function handleNewContact(type: ContactType) {
    const setContacts =
      type === ContactType.Business ? setBusinessContacts : setSecurityContacts;

    setContacts(contacts => [
      ...contacts,
      {
        name: '',
        accountId: '',
        verifyToken: (Math.random() * 1000).toString(),
        email: '',
        contactType: type,
        verification: ContactVerification.Pending,
        draft: true,
        recentlyInvited: false,
      },
    ]);
  }

  async function handleDelete(verifyToken: string, type: ContactType) {
    setDeletingTokenId(null);

    const contacts =
      type === ContactType.Business ? businessContacts : securityContacts;

    const run =
      type === ContactType.Business ? businessDeleteRun : securityDeleteRun;

    const setContacts =
      type === ContactType.Business ? setBusinessContacts : setSecurityContacts;

    const contact = contacts.find(c => c.verifyToken === verifyToken);
    // draft contacts only exist in the UI state and can be removed
    // withou an API call
    if (contact.draft) {
      setContacts(contacts =>
        contacts.filter(c => c.verifyToken !== verifyToken)
      );
      return;
    }

    const removeDeletedContact = (contacts: FormContact[]) =>
      contacts.filter(c => c.verifyToken !== verifyToken);

    const [, err] = await run(verifyToken, type);
    if (!err) {
      setContacts(removeDeletedContact);
    }
  }

  function handleInvite(verifyToken: string, type: ContactType) {
    const contacts =
      type === ContactType.Business ? businessContacts : securityContacts;

    const newContact = contacts.find(c => c.verifyToken === verifyToken);
    // only draft contacts should be invited
    if (!newContact?.draft) {
      return;
    }

    const run =
      type === ContactType.Business ? businessCreateRun : securityCreateRun;
    const setContacts =
      type === ContactType.Business ? setBusinessContacts : setSecurityContacts;

    run(newContact.email, type).then(([ret, err]) => {
      if (err) {
        return;
      }

      setContacts(prevContacts =>
        prevContacts
          .map(contact => {
            if (contact.verifyToken === verifyToken) {
              // Replace the draft contact with the actual contact
              return { ...ret, draft: false, recentlyInvited: true };
            }
            if (contact.verifyToken === ret.verifyToken) {
              // Return `null` if an old contact with this token exists.
              // We'll filter `null` records out next
              return null;
            }
            return contact;
          })
          .filter(contact => contact !== null)
      );
    });
  }

  if (!access.list) {
    return null;
  }

  return (
    <>
      <ContactBox
        title="Security Contacts"
        icon={<Icons.Lock />}
        listAttempt={listAttempt}
        text={`
          Used for notices of important security patches and
          vulnerabilities. Your Teleport account can have up to
          ${MAX_CONTACTS} security contacts.
          `}
        contacts={securityContacts}
        onChange={(verifyToken, val) =>
          handleChange(verifyToken, val, ContactType.Security)
        }
        onNewContact={() => handleNewContact(ContactType.Security)}
        onDelete={verifyToken =>
          setDeletingTokenId({ verifyToken, type: ContactType.Security })
        }
        onInvite={verifyToken =>
          handleInvite(verifyToken, ContactType.Security)
        }
        hasWritePermissions={access.create || access.remove}
        createAttempt={securityCreateAttempt}
        deleteAttempt={securityDeleteAttempt}
      />
      {deletingTokenId !== null && (
        <DeleteDialog
          verifyToken={deletingTokenId.verifyToken}
          type={deletingTokenId.type}
          onClose={() => setDeletingTokenId(null)}
          onDelete={() =>
            handleDelete(deletingTokenId.verifyToken, deletingTokenId.type)
          }
        />
      )}
      <ContactBox
        title="Business Contacts"
        text={`
            Used for account and billing notices. Your Teleport account can have up to
            ${MAX_CONTACTS} business contacts.
          `}
        icon={<Icons.Checks />}
        listAttempt={listAttempt}
        contacts={businessContacts}
        onChange={(verifyToken, val) =>
          handleChange(verifyToken, val, ContactType.Business)
        }
        onNewContact={() => handleNewContact(ContactType.Business)}
        onDelete={verifyToken =>
          setDeletingTokenId({ verifyToken, type: ContactType.Business })
        }
        onInvite={verifyToken =>
          handleInvite(verifyToken, ContactType.Business)
        }
        hasWritePermissions={access.create || access.remove}
        createAttempt={businessCreateAttempt}
        deleteAttempt={businessDeleteAttempt}
      />
    </>
  );
}

type ContactBoxProps = {
  title: string;
  text: string;
  icon: JSX.Element;
  listAttempt: Attempt<Contact[]>;
  contacts: FormContact[];
  onChange: (verifyToken: string, val: string) => void;
  hasWritePermissions: boolean;
  onNewContact: () => void;
  onDelete: (tokenID: string) => void;
  onInvite: (tokenID: string) => void;
  deleteAttempt: Attempt<void>;
  createAttempt: Attempt<Contact>;
};

function ContactBox({
  title,
  text,
  icon,
  listAttempt,
  contacts,
  onChange,
  hasWritePermissions,
  onNewContact,
  onInvite,
  onDelete,
  deleteAttempt,
  createAttempt,
}: ContactBoxProps) {
  return (
    <SupportSectionCard title={title} icon={icon}>
      <Subtitle2 mb={4}>{text}</Subtitle2>
      {listAttempt.status === 'error' && (
        <ComposedAlert
          my="2"
          kind="danger"
          title="Could not list contacts"
          description={listAttempt.statusText}
        />
      )}
      {listAttempt.status !== 'error' && (
        <ContactEmails
          contacts={contacts}
          maxContacts={MAX_CONTACTS}
          onChange={onChange}
          onNewContact={onNewContact}
          onDelete={onDelete}
          onInvite={onInvite}
          writePermissions={hasWritePermissions}
          listAttempt={listAttempt}
          createAttempt={createAttempt}
          deleteAttempt={deleteAttempt}
        />
      )}
    </SupportSectionCard>
  );
}

type DeleteDialogProps = {
  verifyToken: string;
  type: ContactType;
  onClose: () => void;
  onDelete: (verifyToken: string, type: ContactType) => void;
};

function DeleteDialog({
  verifyToken,
  type,
  onClose,
  onDelete,
}: DeleteDialogProps) {
  return (
    <ComposedDialog
      open={true}
      onOpenChange={({ open }) => {
        if (!open) {
          onClose();
        }
      }}
    >
      <Dialog.Header>
        <Dialog.Title>Delete Contact?</Dialog.Title>
      </Dialog.Header>
      <Dialog.Body>
        Are you sure you want to delete this{' '}
        {type === ContactType.Business ? 'business' : 'security'} contact?
      </Dialog.Body>
      <Dialog.Footer>
        <ButtonWarning
          mr="4"
          disabled={false}
          onClick={() => onDelete(verifyToken, type)}
        >
          Delete
        </ButtonWarning>
        <ButtonSecondary disabled={false} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </Dialog.Footer>
    </ComposedDialog>
  );
}
