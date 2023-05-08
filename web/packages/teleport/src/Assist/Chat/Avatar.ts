import styled from 'styled-components';

export const AvatarContainer = styled.div`
  display: flex;
  align-items: center;
  color: rgba(0, 0, 0, 0.5);

  strong {
    display: block;
    margin-right: 10px;
    color: rgba(0, 0, 0, 0.9);
  }
`;

export const ChatItemAvatarImage = styled.div<{ backgroundImage: string }>`
  background: url(${p => p.backgroundImage}) no-repeat;
  width: 22px;
  height: 22px;
  overflow: hidden;
  background-size: cover;
`;

export const ChatItemAvatarTeleport = styled.div`
  background: ${props => props.theme.colors.brand};
  padding: 4px;
  border-radius: 10px;
  left: 0;
  right: auto;
  margin-right: 10px;
`;
