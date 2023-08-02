import React from 'react';

import { ButtonPrimary, Text } from 'design';
import { EnvelopeOpen } from 'design/Icon';

import { useAccessRequestsButton } from 'teleterm/ui/StatusBar/useAccessRequestCheckoutButton';

export function AccessRequestCheckoutButton() {
  const { toggleAccessRequestBar, getPendingResourceCount, isCollapsed } =
    useAccessRequestsButton();
  const count = getPendingResourceCount();

  if (count > 0 && isCollapsed()) {
    return (
      <ButtonPrimary
        onClick={toggleAccessRequestBar}
        px={2}
        size="small"
        title="Toggle Access Request Checkout"
      >
        <EnvelopeOpen mr={2} size="small" color="buttons.primary.text" />
        <Text fontSize="12px">{count}</Text>
      </ButtonPrimary>
    );
  }
  return null;
}
