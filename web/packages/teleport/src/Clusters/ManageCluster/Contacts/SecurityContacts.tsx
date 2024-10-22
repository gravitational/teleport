import React from 'react';

import { Flex, H2, Text } from 'design';
import { MultiRowBox } from 'design/MultiRowBox';
import * as Icons from 'design/Icon';

import { ContactList, ContactListProps } from './ContactList';
import { StyledIconBox, StyledRow } from './BusinessContacts';

type SecurityContactsProps = ContactListProps;

export function SecurityContacts(props: SecurityContactsProps) {
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
            <H2 my="2">Security Contacts</H2>
            <Text>
              Used for notices of important security patches and
              vulnerabilities. Your Teleport account can have up-to 3 security
              contacts.
            </Text>
          </Flex>
          <ContactList {...props} />
        </Flex>
      </StyledRow>
    </MultiRowBox>
  );
}
