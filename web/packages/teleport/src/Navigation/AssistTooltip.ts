import styled from 'styled-components';

import icon from 'teleport/Navigation/teleport-icon.png';

export const TooltipLogosSpacer = styled.div`
  padding: 0 8px;
`;

export const TeleportIcon = styled.div`
  background: url(${icon}) no-repeat;
  width: 30px;
  height: 30px;
  background-size: contain;
  filter: invert(${p => (p.light ? '100%' : '0%')});
`;

export const TooltipLogos = styled.div`
  display: flex;
  align-items: center;
`;

export const TooltipFooter = styled.div`
  margin-top: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

export const Tooltip = styled.div`
  position: absolute;
  z-index: 100;
  top: 150px;
  left: 210px;
  background: ${({ theme }) => theme.colors.levels.popout};
  border-radius: 5px;
  width: 270px;
  font-size: 15px;
  padding: 20px 20px 15px;
  display: flex;
  flex-direction: column;

  &:after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
    border-width: 10px 10px 10px 0;
    border-color: transparent ${({ theme }) => theme.colors.levels.popout}
      transparent transparent;
    left: -10px;
    top: 20px;
  }
`;

export const TooltipTitle = styled.div`
  font-size: 18px;
  font-weight: bold;
  border-radius: 5px;
  margin-bottom: 15px;
`;

export const TooltipTitleBackground = styled.span`
  background: linear-gradient(-45deg, #ee7752, #e73c7e);
  padding: 5px;
  border-radius: 5px;
  color: white;
`;

export const TooltipButton = styled.div`
  cursor: pointer;
  display: inline-flex;
  border: 1px solid ${({ theme }) => theme.colors.text.slightlyMuted};
  border-radius: 5px;
  padding: 8px 15px;
`;
