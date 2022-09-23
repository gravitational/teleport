import styled from 'styled-components';

export const StepContainer = styled.div`
  width: 100%;
  display: flex;
  overflow-x: hidden;
  padding-bottom: 50px;
  margin-top: -24px;
  padding-top: 24px;
`;

export const StepTitle = styled.div`
  display: inline-flex;
  align-items: center;
  transition: 0.2s ease-in opacity;
  cursor: pointer;
  font-size: 18px;
  margin-bottom: 30px;
`;

export const StepTitleIcon = styled.div`
  font-size: 30px;
  margin-right: 20px;
`;

export const StepContent = styled.div`
  display: flex;
  flex: 1;
  flex-direction: column;
  margin-right: 30px;
`;

export const StepAnimation = styled.div`
  flex: 0 0 600px;
  margin-left: 30px;
`;

export const StepInstructions = styled.div``;
