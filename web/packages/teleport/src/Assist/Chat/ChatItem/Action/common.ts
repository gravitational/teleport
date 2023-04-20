import styled from 'styled-components';
import { Type } from 'teleport/Assist/services/messages';

export const Container = styled.div`
  background: rgba(0, 0, 0, 0.2);
  border-radius: 10px;
  padding: 15px 20px;
  position: relative;
  width: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
`;

export const Title = styled.div`
  font-size: 15px;
  margin-bottom: 10px;
`;

export const Items = styled.div`
  display: flex;
  flex-wrap: wrap;
  margin-top: -10px;

  > * {
    margin-top: 10px;
  }
`;

export function getTextForType(type: Type) {
  switch (type) {
    case Type.ExecuteRemoteCommand:
      return 'Connect to';
    case Type.Exec:
      return 'Execute';
    case Type.Message:
      return '';
  }
}
