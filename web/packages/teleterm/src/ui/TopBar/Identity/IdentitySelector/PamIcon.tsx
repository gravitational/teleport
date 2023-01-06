import React from 'react';
import styled from 'styled-components';
import { Image } from 'design';

import pam from './pam.svg';

export function PamIcon() {
  return (
    <PamCircle>
      <Image src={pam} width="14px" />
    </PamCircle>
  );
}

const PamCircle = styled.div`
  height: 24px;
  width: 24px;
  display: flex;
  align-content: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
`;
