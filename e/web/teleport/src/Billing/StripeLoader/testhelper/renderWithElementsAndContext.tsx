import React from 'react';
import { loadStripe } from '@stripe/stripe-js';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { render } from 'design/utils/testing';
import { Elements } from '@stripe/react-stripe-js';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

export const renderWithElementsAndContext = (ui: React.ReactElement) => {
  const sp = loadStripe('');
  const ctx = createTeleportContext();

  return render(
    <Elements stripe={sp}>
      <TeleportContextProvider ctx={ctx}>{ui}</TeleportContextProvider>
    </Elements>
  );
};
