import React, { type JSX } from 'react';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import { Button } from 'design/Button';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import Label from 'design/Label';
import { ShimmerBox } from 'design/ShimmerBox';
import Text, { P3 } from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';
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
    <Flex gap="2" justifyContent="start" flexDirection="column" width="100%">
      {deleteAttempt.status === 'error' && (
        <Danger mb="1" details={deleteAttempt.statusText}>
          Could not delete contact
        </Danger>
      )}
      {createAttempt.status === 'error' && (
        <Danger mb="1" details={createAttempt.statusText}>
          Could not create contact
        </Danger>
      )}

      <Box>Email Address</Box>
      {listAttempt.status === 'processing' && (
        <LoadingSkeleton
          count={3}
          Element={<ShimmerBox height="36px" mb="3" />}
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
        <HoverTooltip tipContent={maxReached ? MAX_LIMIT_TOOLTIP : ''}>
          <Button
            width="fit-content"
            px="3"
            disabled={isProcessing || contacts.length >= maxContacts}
            onClick={onNewContact}
          >
            <Icons.Plus size="small" mr="1" />
            Add New
          </Button>
        </HoverTooltip>
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
          <Flex gap="3" width="100%" alignItems="start">
            <InputFieldWithVerificationState
              contact={contact}
              onChange={onChange}
              disabled={disabled}
              recentlyInvited={recentlyInvited}
              existingEmails={existingEmails}
            />
            {writePermissions && (
              <Box mb="3">
                {!contact.draft && (
                  <HoverTooltip
                    tipContent={deleteDisabled ? DISABLED_DELETED_TOOLTIP : ''}
                  >
                    <EmailButton
                      disabled={disabled || deleteDisabled}
                      text="Delete"
                      icon={<Icons.Trash size="small" />}
                      intent="danger"
                    />
                  </HoverTooltip>
                )}

                {contact.draft && (
                  <EmailButton
                    disabled={disabled}
                    text="Invite"
                    icon={<Icons.PaperPlane size="small" />}
                    intent="primary"
                  />
                )}
              </Box>
            )}
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
    <Box
      width="100%"
      mb="1"
      css={`
        position: relative;
      `}
    >
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
          is valid for two weeks.
        </P3>
      )}
      {/* To properly display the status badge,
          we'll position a box on top of the input, then render
          the same text (the contact email) with `vilisibily: hidden`
          so the status badge is positioned at the right of the input text.
      */}
      <Box
        css={`
          position: absolute;
          top: 0;
          left: 0;
          right: 0;
          height: 40px;
          align-items: center;
          padding-left: 48px;
          pointer-events: none;
          display: flex;
        `}
      >
        {!contact.draft && (
          <>
            <Box
              css={`
                visibility: hidden;
                overflow: hidden;
                max-width: 100%;
              `}
            >
              {contact.email}
            </Box>
            <Box ml="1" mr="3">
              <VerificationStateBadge verification={contact.verification} />
            </Box>
          </>
        )}
      </Box>
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

function EmailButton({
  disabled,
  text,
  icon,
  intent,
}: {
  disabled: boolean;
  text: string;
  icon: JSX.Element;
  intent: 'danger' | 'primary';
}) {
  return (
    <Button
      height="40px"
      intent={intent}
      fill="border"
      disabled={disabled}
      type="submit"
      css={`
        padding-left: ${p => p.theme.space[2]}px;
        padding-right: ${p => p.theme.space[2]}px;
        @media screen and (min-width: ${p => p.theme.breakpoints.medium}) {
          width: 120px;
          padding-left: ${p => p.theme.space[3]}px;
          padding-right: ${p => p.theme.space[3]}px;
        }
      `}
    >
      {icon}
      <Text
        ml="1"
        css={`
          display: none;
          @media screen and (min-width: ${p => p.theme.breakpoints.medium}) {
            display: inline;
          }
        `}
        as="span"
      >
        {text}
      </Text>
    </Button>
  );
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
