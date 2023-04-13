import styled from 'styled-components';

export interface CommonInstructionsProps {
  onNext: () => void;
  clusterPublicUri: string;
}

export const InstructionsContainer = styled.div`
  flex: 0 0 600px;
  padding-right: 100px;
`;
