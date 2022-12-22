import styled from 'styled-components';

export const WindowContainer = styled.div`
  border-radius: 5px;
  width: 100%;
  box-shadow: 0px 0px 20px 0px rgba(0, 0, 0, 0.43);
`;

export const WindowTitleBarContainer = styled.div`
  background: #040b1d;
  height: 32px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  border-top-left-radius: 5px;
  border-top-right-radius: 5px;
`;

export const WindowTitleBarButtons = styled.div`
  display: flex;
  position: absolute;
  top: 50%;
  left: 10px;
  transform: translate(0, -50%);
`;

export const WindowTitleBarButton = styled.div`
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-right: 5px;
`;

export const WindowContentContainer = styled.div`
  background: #04162c;
  height: var(--content-height, 660px);
  overflow-y: auto;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
`;

export const WindowCode = styled.div`
  font-size: 12px;
  font-family: Menlo, DejaVu Sans Mono, Consolas, Lucida Console, monospace;
  line-height: 20px;
  white-space: pre-wrap;
`;
