import { createSystem, defineConfig } from '@chakra-ui/react';

const config = defineConfig({
  theme: {
    tokens: {
      colors: {},
    },
  },
});

export const system = createSystem(config);

export { system as default }; // for the Chakra CLI
