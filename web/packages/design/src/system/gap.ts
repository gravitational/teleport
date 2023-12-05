import { style } from 'styled-system';

export interface GapProps {
  gap?: string | number;
}

export const gap = style<number | string>({
  prop: 'gap',
  cssProperty: 'gap',
  // This makes gap use the space defined in the theme.
  // https://github.com/styled-system/styled-system/blob/v3.1.11/src/index.js#L67
  key: 'space',
});
