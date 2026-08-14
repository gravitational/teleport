import { QueryClientProvider } from '@tanstack/react-query';
import { PropsWithChildren } from 'react';

import { testQueryClient } from 'design/utils/testing';

import { Provider } from './Provider';

export function ProviderWithQuery({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={testQueryClient}>
      <Provider>{children}</Provider>
    </QueryClientProvider>
  );
}
