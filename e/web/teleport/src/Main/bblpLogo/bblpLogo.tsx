import React from 'react';

import { Image } from 'design';

import bblpLogo from './bblpLogo.svg';

export const BblpLogo = () => {
  return (
    <Image
      src={bblpLogo}
      height="32px"
      width="fit-content"
      style={{
        marginTop: '28px',
        marginLeft: '32px',
        marginBottom: '12px',
      }}
      alt="bblp logo"
    />
  );
};
