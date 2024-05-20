// Note: this file might have been named "styled-components", but it couldn't:
// naming it this way completely breaks Storybook.
import * as styled from 'styled-components';
import { ThemedStyledComponentsModule } from 'styled-components';

import { Theme } from './theme/themes/types';

const {
  default: defaultStyled,
  createGlobalStyle,
  css,
  useTheme,
} = styled as ThemedStyledComponentsModule<Theme>;

export { createGlobalStyle, css, useTheme };
export default defaultStyled;
