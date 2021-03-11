import React from 'react';
import { Card, Box, Text, Flex, ButtonPrimary } from 'design';
import { Account, countryMap } from 'e-teleport/services/cloud';

export default function AccountInfo(props: Props) {
  const account = props.account;
  const contactEmail = account.contactEmail || 'Empty';
  return (
    <Card p={5} pt={4} maxWidth={props.maxWidth}>
      <Flex alignItems="center" mb={2} justifyContent="space-between">
        <Text typography="h4" bold>
          Billing Information
        </Text>
        <ButtonPrimary size="small" onClick={props.onEdit}>
          Edit
        </ButtonPrimary>
      </Flex>
      <Box>
        <Text>{contactEmail}</Text>
        <Text>{account.companyName}</Text>
        <Text>{account.companyAddressLine1}</Text>
        <Text>{account.companyAddressCity}</Text>
        <Text>{account.companyAddressState}</Text>
        <Text>{countryMap[account.companyAddressCountry]}</Text>
        <Text>{account.companyAddressPostalCode}</Text>
      </Box>
    </Card>
  );
}

type Props = {
  onEdit(): void;
  account: Account;
  maxWidth?: string;
};
