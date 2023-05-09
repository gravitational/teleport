import React, { useState } from 'react';

import { Box, ButtonSecondary, Flex, Text } from 'design';

import { useTheme } from 'styled-components';
import * as Icons from 'design/Icon';

import ErrorMessage from 'teleport/components/AgentErrorMessage';

import { CardsListProps } from 'e-teleport/Billing/types';
import { StripeCard } from 'e-teleport/services/cloud';
import { PaymentDeleteDialog } from 'e-teleport/Billing/Payment/PaymentDeleteDialog';
import { PaymentAddDialog } from 'e-teleport/Billing/Payment/PaymentAddDialog';
import useTeleport from 'e-teleport/useTeleportE';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';

export const CardList = ({
  cards,
  setPageState,
  stripeMissingPaymentMethod,
  defaultSourceID = '',
}: CardsListProps) => {
  const theme = useTheme();
  const ctx = useTeleport();

  const [openAdd, setOpenAdd] = useState<boolean>(false);
  const [openDelete, setOpenDelete] = useState<boolean>(false);
  const [selectedCard, setSelectedCard] = useState<StripeCard>(null);
  const [networkState, setNetworkState] = useState<NetworkState>({});

  // reload refreshes the PaymentsAndInvoices state without reloading stripe via StripeLoader.reload
  const reload = (): void => {
    ctx.cloudService.fetchPaymentsAndInvoices().then(resp => {
      setPageState(resp);
      setNetworkState({ status: 'success', error: undefined });
    });
  };

  const getIcon = (brand: string): React.ReactNode => {
    switch (brand) {
      case 'visa':
        return <Icons.Visa fontSize="20px" data-testid={'icon-visa'} />;
      case 'amex':
        return <Icons.Amex fontSize="20px" data-testid={'icon-amex'} />;
      case 'mastercard':
        return (
          <Icons.MasterCard fontSize="20px" data-testid={'icon-mastercard'} />
        );
      case 'discover':
        return <Icons.Discover fontSize="20px" data-testid={'icon-discover'} />;
      case 'cartes_bancaires':
      case 'diners':
      case 'jcb':
      case 'unionpay':
        // generic known/supported card brands
        return <Icons.Stripe fontSize="20px" data-testid={'icon-generic'} />;
      default:
        // unsupported card brand
        return <Icons.Cross fontSize="20px" data-testid={'icon-unknown'} />;
    }
  };

  const makeDefault = (card: StripeCard): void => {
    setNetworkState({ status: 'loading', error: undefined });

    ctx.cloudService
      .updateCard({
        prevCardId: card.id,
        nextCardId: card.id,
        isDefault: true,
      })
      .then(() => {
        reload();
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  // todo (michellescripts) update delete default card - give new default options as part of design https://github.com/gravitational/cloud/issues/3536;
  const getCardBox = (card: StripeCard): React.ReactElement => {
    const defaultCard = card.id === defaultSourceID;

    return (
      <Box
        key={card.id}
        width="300px"
        bg={theme.colors.spotBackground[1]}
        borderRadius="12px"
        m="20px 0 0 0"
        p="20px 20px 20px 40px"
      >
        <Flex justifyContent="space-between">
          {getIcon(card.brand)}
          {defaultCard && <Icons.Check fontSize="20px" color="green" />}
        </Flex>
        <Text>{card.name ? card.name : <br />}</Text>
        <Text>**** **** **** {card.last4}</Text>
        <Text>
          {card.expirationMonth}/{card.expirationYear}
        </Text>
        <Flex gap="8px" mt="12px">
          <ButtonSecondary
            disabled={defaultCard || networkState.status == 'loading'}
            onClick={() => {
              makeDefault(card);
            }}
          >
            Make Default
          </ButtonSecondary>
          <ButtonSecondary
            disabled={
              cards.length === 1 ||
              defaultCard ||
              networkState.status == 'loading'
            }
            onClick={() => {
              setSelectedCard(card);
              setOpenDelete(true);
            }}
          >
            Delete
          </ButtonSecondary>
        </Flex>
      </Box>
    );
  };

  return (
    <>
      <h3>Payment Methods</h3>
      <Text color="text.secondary">
        At most, three credit cards can be added.
      </Text>
      <Flex gap="8px">
        {cards.map(c => getCardBox(c))}
        {cards.length < 3 && (
          <Box
            key="add-payment"
            width="300px"
            borderRadius="12px"
            m="20px 0 0 0"
            p="auto"
          >
            <ButtonSecondary
              height="100%"
              width="100%"
              disabled={networkState.status == 'loading'}
              onClick={() => setOpenAdd(true)}
            >
              <Flex
                justifyContent="center"
                alignItems="center"
                flexDirection="column"
                height="100%"
              >
                <Icons.Add fontSize="40px" />
                <Text>Add a Payment Method</Text>
              </Flex>
            </ButtonSecondary>
          </Box>
        )}
        {openAdd && (
          <PaymentAddDialog
            open={openAdd}
            setOpen={setOpenAdd}
            reload={reload}
            title={`Add Payment Method`}
            showDefaultOption={true}
            stripeMissingPaymentMethod={stripeMissingPaymentMethod}
          />
        )}
        {openDelete && (
          <PaymentDeleteDialog
            open={openDelete}
            setOpen={setOpenDelete}
            card={selectedCard}
            reload={reload}
          />
        )}
      </Flex>
      {networkState.error != undefined && (
        <ErrorMessage message={networkState.error.message} />
      )}
    </>
  );
};
