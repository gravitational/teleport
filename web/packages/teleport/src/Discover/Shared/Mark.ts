import styled from 'styled-components';

export const Mark = styled.mark`
  padding: 2px 5px;
  border-radius: 6px;
  background-color: ${props =>
    props.light ? '#d3d3d3' : 'rgb(255 255 255 / 17%)'};
  color: inherit;
`;
