import React, { useEffect } from 'react';
import { useLocation } from 'react-router';

import { CardSuccess } from 'design';

export const SSOConfirm = () => {
  const { search } = useLocation();
  const params = new URLSearchParams(search);
  const mfaToken = params.get('response');

  useEffect(() => {
    const bc = new BroadcastChannel('sso_confirm');
    if (mfaToken) {
      bc.postMessage({ mfaToken });
      setTimeout(() => {
        window.close();
      }, 1000);
    }

    return () => {
      // bc.removeEventListener('message', handleMessage);
      // bc.close();
    };
  }, [mfaToken]);
  return (
    <CardSuccess title="Authenticated">
      You have successfully authenticated
    </CardSuccess>
  );
};
