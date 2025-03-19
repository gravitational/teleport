import { ChakraProvider } from '@chakra-ui/react';
import type { PropsWithChildren } from 'react';

import { system } from '../system';

export function ThemeProvider({ children }: PropsWithChildren) {
  return <ChakraProvider value={system}>{children}</ChakraProvider>;
}
