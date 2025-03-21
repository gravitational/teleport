import { defineTokens } from '@chakra-ui/react';

import { getPlatform, Platform } from 'shared-new/platform';

const fontFamily = `Ubuntu2, -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol";`;

const fontMonoLinux = `"Droid Sans Mono", "monospace", monospace, "Droid Sans Fallback"`;
const fontMonoWin = `Consolas, "Courier New", monospace`;
const fontMonoMac = `Menlo, Monaco, "Courier New", monospace`;

export const fonts = defineTokens.fonts({
  heading: {
    value: fontFamily,
  },
  body: {
    value: fontFamily,
  },
  mono: {
    value: getMonoFont(),
  },
});

function getMonoFont() {
  const platform = getPlatform();

  if (platform === Platform.Linux) {
    return fontMonoLinux;
  }

  if (platform === Platform.macOS) {
    return fontMonoMac;
  }

  if (platform === Platform.Windows) {
    return fontMonoWin;
  }

  return fontMonoLinux;
}
