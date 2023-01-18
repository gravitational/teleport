import { useState } from 'react';
import { isPrivateKeyRequiredError } from 'shared/utils/errorType';

import type { PrivateKeyAccessRequest } from 'teleport/components/PrivateKeyPolicy';

export function usePrivateKeyAccessRequest() {
  const [privateKeyRequirement, setPrivateKeyRequirement] =
    useState<PrivateKeyAccessRequest>();

  function updatePrivateKeyRequirement(params: PrivateKeyAccessRequest) {
    setPrivateKeyRequirement(params);
  }

  function clearPrivateKeyRequirement() {
    setPrivateKeyRequirement(null);
  }

  return {
    privateKeyRequirement,
    updatePrivateKeyRequirement,
    clearPrivateKeyRequirement,
    isPrivateKeyRequiredError,
  };
}
