import React, { useMemo, useState } from 'react';

import { Box, ButtonSecondary, Flex, Text } from 'design';

import { useTheme } from 'styled-components';
import * as Icons from 'design/Icon';

import ErrorMessage from 'teleport/components/AgentErrorMessage';

import Label from 'design/Label';

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

  /**
   * Returns cards with the default card in the zero index
   */
  const sortedCards = useMemo(() => {
    if (!defaultSourceID || !cards || cards.length == 0) {
      return cards;
    }

    const index = cards.findIndex(card => card.id == defaultSourceID);
    if (index === -1) {
      return cards;
    }

    const defaultCard = cards[index];
    cards.splice(index, 1);
    cards.unshift(defaultCard);

    return cards;
  }, [defaultSourceID, cards]);

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
        return <Icons.CCVisa size="small" data-testid={'icon-visa'} />;
      case 'amex':
        return <Icons.CCAmex size="small" data-testid={'icon-amex'} />;
      case 'mastercard':
        return (
          <Icons.CCMasterCard size="small" data-testid={'icon-mastercard'} />
        );
      case 'discover':
        return <Icons.CCDiscover size="small" data-testid={'icon-discover'} />;
      case 'cartes_bancaires':
      case 'diners':
      case 'jcb':
      case 'unionpay':
        // generic known/supported card brands
        return <Icons.CCStripe size="small" data-testid={'icon-generic'} />;
      default:
        // unsupported card brand
        return <Icons.Cross size="small" data-testid={'icon-unknown'} />;
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
        width="320px"
        bg={theme.colors.levels.surface}
        borderRadius="4px"
        m="20px 0 0 0"
        p="20px 20px 20px 40px"
      >
        <Flex justifyContent="space-between">
          {getIcon(card.brand)}
          {defaultCard && (
            <Label kind="success" data-testid="default-card">
              default
            </Label>
          )}
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
              sortedCards.length === 1 ||
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
        {sortedCards.map(c => getCardBox(c))}
        {sortedCards.length < 3 && (
          <Box
            key="add-payment"
            width="320px"
            borderRadius="4px"
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
                <Icons.Add size="small" mb={3} />
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
            title={`Add a New Payment Method`}
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
        <Box mt={2} maxWidth="920px">
          <ErrorMessage message={networkState.error.message} />
        </Box>
      )}
    </>
  );
};
