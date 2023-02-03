import { ConfigServiceProvider } from '../types';

export type AppearanceConfig = {
  fonts: {
    sansSerif: string;
    mono: string;
  };
};

export const appearanceConfigProvider: ConfigServiceProvider<AppearanceConfig> =
  {
    getDefaults(platform) {
      switch (platform) {
        case 'win32':
          return {
            fonts: {
              sansSerif: "system-ui, 'Segoe WPC', 'Segoe UI', sans-serif",
              mono: "'Consolas', 'Courier New', monospace",
            },
          };
        case 'linux':
          return {
            fonts: {
              sansSerif: "system-ui, 'Ubuntu', 'Droid Sans', sans-serif",
              mono: "'Droid Sans Mono', 'Courier New', monospace, 'Droid Sans Fallback'",
            },
          };
        case 'darwin':
          return {
            fonts: {
              sansSerif: '-apple-system, BlinkMacSystemFont, sans-serif',
              mono: "Menlo, Monaco, 'Courier New', monospace",
            },
          };
      }
    },
  };
