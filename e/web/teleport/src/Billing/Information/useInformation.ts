import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';
import { BillingInformation, Account } from 'e-teleport/services/cloud';
/*
    disable below rule as import/named rule and babel ts parser do not
    handle re-exported TS types
*/
/* eslint import/named : 0 */
import { Stripe, CreatePaymentMethodData } from '@stripe/stripe-js';

export default function useAccount(ctx: TeleportContextE) {
  const initAttempt = useAttempt('processing');
  const cardListAttempt = useAttempt('');
  const cardDialogAttempt = useAttempt('');
  const accountEditorAttempt = useAttempt('');
  const [info, setInfo] = useState<BillingInformation>(null);
  const [isAccountEditorVisible, setAccountEditorVisible] = useState(false);
  const [isCardDialogVisible, setCardDialogVisible] = useState(false);

  useEffect(() => {
    initAttempt.run(() =>
      ctx.cloudService.fetchBillingInformation().then(json => {
        setInfo(json);
      })
    );
  }, []);

  function toggleAccountEditor() {
    setAccountEditorVisible(!isAccountEditorVisible);
  }

  function updateAccount(acc: Account) {
    accountEditorAttempt.run(() =>
      ctx.cloudService
        .updateAccount({ account: acc })
        .then(refresh)
        .then(() => {
          toggleAccountEditor();
        })
    );
  }

  function refresh() {
    return ctx.cloudService.fetchBillingInformation().then(json => {
      setInfo(json);
    });
  }

  function removeCard(paymentMethodId: string) {
    cardListAttempt.run(() =>
      ctx.cloudService
        .removeCard({
          cardId: paymentMethodId,
        })
        .then(refresh)
    );
  }

  function addCard(stripe: Stripe, data: CreatePaymentMethodData) {
    cardDialogAttempt.run(() =>
      createStripePaymentMethod(stripe, data).then(paymentMethodId =>
        ctx.cloudService
          .addCard({
            cardId: paymentMethodId,
            isDefault: !info.defaultPaymentMethodId,
          })
          .then(refresh)
          .then(() => setCardDialogVisible(false))
      )
    );
  }

  function setDefaultCard(cardId: string) {
    cardListAttempt.run(() =>
      ctx.cloudService
        .updateCard({
          prevCardId: cardId,
          nextCardId: cardId,
          isDefault: true,
        })
        .then(refresh)
    );
  }

  return {
    initAttempt: initAttempt.attempt,
    cardListAttempt: cardListAttempt.attempt,
    cardDialogAttempt: cardDialogAttempt.attempt,
    accountEditorAttempt: accountEditorAttempt.attempt,
    addCard,
    info,
    isCardDialogVisible,
    removeCard,
    setCardDialogVisible,
    setDefaultCard,
    isAccountEditorVisible,
    toggleAccountEditor,
    updateAccount,
  };
}

export type State = ReturnType<typeof useAccount>;

function createStripePaymentMethod(
  stripe: Stripe,
  data: CreatePaymentMethodData
) {
  return stripe.createPaymentMethod(data).then(result => {
    if (result.error) {
      return Promise.reject(new Error(result.error.message));
    }

    return result.paymentMethod.id;
  });
}
