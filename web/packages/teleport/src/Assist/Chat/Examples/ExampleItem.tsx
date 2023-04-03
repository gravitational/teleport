import styled, { keyframes } from 'styled-components';

const appear = keyframes`
  0% {
    opacity: 0;
  }

  100% {
    opacity: 1;
  }
`;

export const ExampleItem = styled.div`
  background: #222c5a;
  margin-right: 20px;
  padding: 10px 15px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  font-size: 16px;
  opacity: 0;
  animation: ${appear} linear 0.6s forwards;

  svg {
    margin-right: 15px;
  }
`;
