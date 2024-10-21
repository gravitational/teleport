import React from 'react';
import styled from 'styled-components';

import { Flex, H2, Text } from 'design';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import * as Icons from 'design/Icon';

import { Contact } from 'teleport/services/contacts/contacts';

import { ContactList } from './ContactList';

type BusinessContactsProps = {
  contacts: Contact[];
  maxContacts: number;
  onSubmit: (contact: Contact) => void;
  onDelete: (contact: Contact) => void;
  isLoading: boolean;
};

export function BusinessContacts({
  contacts,
  maxContacts,
  onSubmit,
  onDelete,
  isLoading,
}: BusinessContactsProps) {
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
            <H2 my="2">Business Contacts</H2>
            <Text>
              Used for account and billing notices. Your Teleport account can
              have up-to 3 business contacts.
            </Text>
          </Flex>
          <ContactList
            contacts={[...contacts]}
            maxContacts={maxContacts}
            onDelete={onDelete}
            onSubmit={onSubmit}
            isLoading={isLoading}
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
