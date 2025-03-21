import { ChakraProvider } from '@chakra-ui/react';
import { ThemeProvider as NextThemeProvider } from 'next-themes';
import type { PropsWithChildren } from 'react';

import { system } from '../theme';

export function ThemeProvider({ children }: PropsWithChildren) {
  return (
    <ChakraProvider value={system}>
      <NextThemeProvider attribute="class" disableTransitionOnChange>{children}</NextThemeProvider>
    </ChakraProvider>
  );
}
