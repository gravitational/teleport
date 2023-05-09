import { Box, ButtonSecondary, Text } from 'design';
import React, { useState } from 'react';
import { useTheme } from 'styled-components';

import * as Icons from 'design/Icon';
import { fromUnixTime } from 'date-fns';

import { displayShortDate } from 'shared/services/loc/loc';

import { PaymentAddDialog } from 'e-teleport/Billing/Payment/PaymentAddDialog';
import { PaymentBannerProps } from 'e-teleport/Billing/types';

export const PaymentBanner = ({
  productName,
  reload,
  stripeTrialEnd,
}: PaymentBannerProps) => {
  const theme = useTheme();
  const trialEndDate = fromUnixTime(stripeTrialEnd);
  const [open, setOpen] = useState<boolean>(false);

  return (
    <>
      <Box bg={theme.colors.spotBackground[1]} m="0 -40px" p="20px 0 20px 40px">
        <h2>Payment Method</h2>
        <Text>
          Required for the Teleport {productName} Plan: Add a default credit
          card below to automatically upgrade to the {productName} plan when
          your trial expires.
        </Text>
        <ButtonSecondary onClick={() => setOpen(true)} mt="12px">
          <Icons.Add />
          &nbsp;Add a Payment Method
        </ButtonSecondary>
      </Box>
      {open && (
        <PaymentAddDialog
          open={open}
          reload={reload}
          setOpen={setOpen}
          makeDefault={true}
          stripeMissingPaymentMethod={true}
          title={`Upgrade to ${productName}`}
          description={`Add a payment method to automatically upgrade your account to the 
          ${productName} plan when your trial ends on ${displayShortDate(
            trialEndDate
          )}`}
        />
      )}
    </>
  );
};
