import styled, { keyframes } from 'styled-components';

const loader = keyframes`
  0% {
    transform: translateX(0);
  }
  50% {
    transform: translateX(1420%);
  }
  100% {
    transform: translateX(0);
  }
`;

const DotsContainer = styled.div`
  position: relative;
  width: 100%;
  padding-top: 6.6%;
`;

const Container = styled.div`
  box-sizing: border-box;
  width: 100px;
  height: 5px;

  *,
  *:before,
  *:after {
    box-sizing: inherit;
  }
`;

const Dot = styled.div`
  width: 6.6%;
  padding-top: 6.6%;
  animation: ${loader} 2s ease-in-out infinite;
  border-radius: 100%;
  display: inline-block;
  position: absolute;
  top: 0;
  left: 0;
`;

export function Dots() {
  let dots = new Array(6).fill('').map((e, index) => {
    return (
      <Dot
        key={index}
        style={{
          backgroundColor: 'white',
          opacity: 0.5,
          animationDelay: `0.${index}s`,
        }}
      ></Dot>
    );
  });

  return (
    <Container>
      <DotsContainer>{dots}</DotsContainer>
    </Container>
  );
}
