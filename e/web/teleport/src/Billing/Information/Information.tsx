import React from 'react';
import { Box, Alert, Indicator } from 'design';
import useTeleportContext from 'e-teleport/useTeleportE';
import useInformation, { State } from './useInformation';
import AccountEditor from './AccountEditor';
import AccountInfo from './AccountInfo';
import CreditCardList from './CreditCardList';
import CreditCardDialog from './CreditCardDialog';

export default function Container() {
  const ctx = useTeleportContext();
  const state = useInformation(ctx);
  return <Information {...state} />;
}

export function Information({
  addCard,
  accountEditorAttempt,
  cardDialogAttempt,
  cardListAttempt,
  info,
  initAttempt,
  isCardDialogVisible,
  removeCard,
  setCardDialogVisible,
  setDefaultCard,
  isAccountEditorVisible,
  toggleAccountEditor,
  updateAccount,
}: State) {
  if (initAttempt.status === 'processing') {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  if (initAttempt.status === 'failed') {
    return <Alert kind="danger" children={initAttempt.statusText} />;
  }

  return (
    <>
      {isAccountEditorVisible && (
        <AccountEditor
          attempt={accountEditorAttempt}
          account={info.account}
          onSave={updateAccount}
          onClose={toggleAccountEditor}
        />
      )}

      {isCardDialogVisible && (
        <CreditCardDialog
          attempt={cardDialogAttempt}
          onSave={addCard}
          onClose={() => setCardDialogVisible(false)}
          stripeKey={info.stripePublicKey}
        />
      )}
      <CreditCardList
        attempt={cardListAttempt}
        defaultPaymentMethodId={info.defaultPaymentMethodId}
        cards={info.cardsList || []}
        maxWidth="716px"
        mb={4}
        onRemove={removeCard}
        onSetDefault={setDefaultCard}
        onNew={() => setCardDialogVisible(true)}
      />
      <AccountInfo
        onEdit={toggleAccountEditor}
        account={info.account}
        maxWidth="716px"
      />
    </>
  );
}
