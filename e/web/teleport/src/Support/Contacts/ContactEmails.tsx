import {
  Box,
  Button,
  ComposedAlert,
  Flex,
  Label,
  P3,
  ShimmerBox,
  Tooltip,
} from '@gravitational/design-system';
import React from 'react';

import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { LoadingSkeleton } from 'shared/components/UnifiedResources/shared/LoadingSkeleton';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredAll,
  requiredEmailLike,
  Rule,
} from 'shared/components/Validation/rules';
import { Attempt } from 'shared/hooks/useAsync';

import {
  Contact,
  ContactVerification,
} from 'e-teleport/services/contacts/types';

const MAX_LIMIT_TOOLTIP =
  'Maximum number of contacts reached. Delete an existing contact to add a new one.';
const DISABLED_DELETED_TOOLTIP =
  'You must have at least one verified contact. To delete this, you must first add and verify another address.';

/**
 * FormContact extends Contact by adding UI-specific values.
 */
export type FormContact = Contact & {
  draft: boolean;
  recentlyInvited: boolean;
};

export type ContactEmailsProps = {
  contacts: FormContact[];
  maxContacts: number;
  onChange: (tokenID: string, email: string) => void;
  onNewContact: () => void;
  onDelete: (tokenID: string) => void;
  onInvite: (tokenID: string) => void;
  writePermissions: boolean;
  listAttempt: Attempt<Contact[]>;
  deleteAttempt: Attempt<void>;
  createAttempt: Attempt<Contact>;
};

export function ContactEmails({
  contacts,
  maxContacts,
  onChange,
  onNewContact,
  onDelete,
  onInvite,
  writePermissions,
  listAttempt,
  deleteAttempt,
  createAttempt,
}: ContactEmailsProps) {
  const numberOfVerifiedContacts = contacts.filter(
    c => c.verification === ContactVerification.Verified
  ).length;

  const maxReached = contacts.length >= maxContacts;

  const isProcessing =
    listAttempt.status === 'processing' ||
    deleteAttempt.status === 'processing' ||
    createAttempt.status === 'processing';

  return (
    <Flex direction="column" width="100%">
      {deleteAttempt.status === 'error' && (
        <ComposedAlert
          mb="1"
          kind="danger"
          title="Could not delete contact"
          description={deleteAttempt.statusText}
        />
      )}
      {createAttempt.status === 'error' && (
        <ComposedAlert
          mb="1"
          kind="danger"
          title="Could not create contact"
          description={createAttempt.statusText}
        />
      )}

      {listAttempt.status === 'processing' && (
        <LoadingSkeleton
          count={3}
          Element={<ShimmerBox height="36px" mb="4" />}
        />
      )}
      {contacts.map(contact => (
        <EmailInput
          key={contact.verifyToken}
          contact={contact}
          onChange={email => onChange(contact.verifyToken, email)}
          disabled={isProcessing}
          onDelete={() => onDelete(contact.verifyToken)}
          deleteDisabled={
            contact.verification === ContactVerification.Verified &&
            numberOfVerifiedContacts <= 1
          }
          onInvite={() => onInvite(contact.verifyToken)}
          writePermissions={writePermissions}
          recentlyInvited={contact.recentlyInvited}
          existingEmails={contacts
            .filter(c => c.verifyToken !== contact.verifyToken)
            .map(c => c.email)}
        />
      ))}
      {writePermissions && (
        <Tooltip disabled={!maxReached} content={MAX_LIMIT_TOOLTIP}>
          <Button
            px="4"
            disabled={isProcessing || contacts.length >= maxContacts}
            onClick={onNewContact}
            fill="border"
            width={{ base: '100%', sm: 'fit-content' }}
          >
            <Icons.Plus size="small" mr="1" />
            Invite New
          </Button>
        </Tooltip>
      )}
    </Flex>
  );
}

type EmailInputProps = {
  contact: FormContact;
  onChange: (email: string) => void;
  onDelete: () => void;
  disabled: boolean;
  deleteDisabled: boolean;
  onInvite: () => void;
  writePermissions: boolean;
  recentlyInvited: boolean;
  existingEmails: string[];
};

function EmailInput({
  contact,
  onChange,
  disabled,
  onDelete,
  deleteDisabled,
  onInvite,
  writePermissions,
  recentlyInvited,
  existingEmails,
}: EmailInputProps) {
  function onSubmit(e: React.FormEvent<HTMLFormElement>, validator: Validator) {
    e.preventDefault();
    // invite
    if (contact.draft) {
      if (!validator.validate()) return;
      onInvite();
      return;
    }
    // delete
    onDelete();
  }
  return (
    <Validation>
      {({ validator }) => (
        <form onSubmit={e => onSubmit(e, validator)}>
          <Flex
            gap={{ base: 2, sm: 4 }}
            width="100%"
            align="start"
            direction={{ base: 'column', sm: 'row' }}
          >
            <Flex width="100%" gap={1}>
              <InputFieldWithVerificationState
                contact={contact}
                onChange={onChange}
                disabled={disabled}
                recentlyInvited={recentlyInvited}
                existingEmails={existingEmails}
              />
              {writePermissions && (
                <Box
                  mb={{ base: 2, sm: 4 }}
                  alignSelf={{ base: 'flex-start', sm: 'auto' }}
                >
                  {!contact.draft && (
                    <Tooltip
                      disabled={!deleteDisabled}
                      content={DISABLED_DELETED_TOOLTIP}
                    >
                      <Button
                        data-testid="delete-contact-btn"
                        height="40px"
                        fill="border"
                        disabled={disabled || deleteDisabled}
                        type="submit"
                        intent="danger"
                        width={{ base: 'fit-content', sm: '40px' }}
                        px={{ base: 2, sm: 4 }}
                      >
                        <Icons.Trash size="small" />
                      </Button>
                    </Tooltip>
                  )}

                  {contact.draft && (
                    <Button
                      height="40px"
                      fill="border"
                      disabled={disabled}
                      type="submit"
                      width="80px"
                      px={{ base: 2, sm: 4 }}
                    >
                      <Icons.PaperPlane size="small" mr={1} />
                      Invite
                    </Button>
                  )}
                </Box>
              )}
            </Flex>
          </Flex>
        </form>
      )}
    </Validation>
  );
}

function InputFieldWithVerificationState({
  contact,
  onChange,
  disabled,
  recentlyInvited,
  existingEmails,
}: {
  contact: FormContact;
  onChange: (email: string) => void;
  disabled: boolean;
  recentlyInvited: boolean;
  existingEmails: string[];
}) {
  return (
    <Box width="100%" mb="1" position="relative">
      <FieldInput
        icon={Icons.EmailSolid}
        width="100%"
        value={contact.email}
        onChange={e => onChange(e.target.value)}
        disabled={disabled || !contact.draft}
        rule={requiredAll(
          requiredEmailLike,
          requiredUnique(existingEmails, 'Email already invited')
        )}
        mb="0"
        placeholder="mail@example.com"
      />
      {recentlyInvited && (
        <P3>
          Invitation sent successfully and is pending verification. Invitation
          is valid for 14 days.
        </P3>
      )}
      {/* To properly display the status badge,
          we'll position a box on top of the input, then render
          the same text (the contact email) with `visibility: hidden`
          so the status badge is positioned at the right of the input text.
      */}
      <Flex
        position="absolute"
        top={0}
        left={0}
        right={0}
        height="40px"
        align="center"
        paddingLeft="48px"
        pointerEvents="none"
      >
        {!contact.draft && (
          <>
            <Box visibility="hidden" overflow="hidden" maxWidth="100%">
              {contact.email}
            </Box>
            <Box ml="1" mr="4">
              <VerificationStateBadge verification={contact.verification} />
            </Box>
          </>
        )}
      </Flex>
    </Box>
  );
}

function VerificationStateBadge({
  verification,
}: {
  verification: ContactVerification;
}) {
  switch (verification) {
    case ContactVerification.Verified:
      return (
        <Label data-testid="verification-state-badge-active" kind="success">
          active
        </Label>
      );
    case ContactVerification.Pending:
      return (
        <Label data-testid="verification-state-badge-pending" kind="warning">
          pending
        </Label>
      );
    case ContactVerification.Expired:
      return (
        <Label data-testid="verification-state-badge-expired" kind="danger">
          expired
        </Label>
      );
    default:
      verification satisfies never;
  }
}

/**
 * checks if the given value is unique among the `values` array.
 * @param values a list of items
 * @param message  the message returned when the item is not unique.
 * Defaults to "Value already included"
 */
const requiredUnique =
  (values: string[], message = 'Value already included'): Rule =>
  (val: string) =>
  () => {
    const included = values.includes(val);

    return included
      ? {
          valid: false,
          message: message,
        }
      : {
          valid: true,
        };
  };
