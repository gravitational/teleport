import styled, { keyframes } from 'styled-components';

const loading = keyframes`
  0% {
    opacity: 0;
  }
  50% {
    opacity: 0.8;
  }
  100% {
    opacity: 0;
  }
`;

export const Typing = styled.div`
  margin: 0 30px 0 30px;
`;

export const TypingContainer = styled.div`
  position: relative;
  padding: 10px;
  display: flex;
`;

export const TypingDot = styled.div`
  width: 8px;
  height: 8px;
  margin-right: 8px;
  background: #8d8c91;
  border-radius: 50%;
  opacity: 0;
  animation: ${loading} 1.5s ease-in-out infinite;
`;
