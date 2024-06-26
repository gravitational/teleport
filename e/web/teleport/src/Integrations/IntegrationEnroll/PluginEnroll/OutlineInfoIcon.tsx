import styled from 'styled-components';
import { Info } from 'design/Icon';

export const OutlineInfoIcon = styled(Info)`
  background-color: ${p => p.theme.colors.link};
  border-radius: 100px;
  height: 32px;
  width: 32px;
  color: ${p => p.theme.colors.text.primaryInverse};
  margin-right: ${p => p.theme.space[2]}px;
`;
