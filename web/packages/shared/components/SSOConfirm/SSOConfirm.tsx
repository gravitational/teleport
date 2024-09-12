import React, { useEffect } from 'react';

import { CardSuccess } from 'design';

export const SSOConfirm = () => {
  useEffect(() => {
    const bc = new BroadcastChannel('sso_confirm');
    function handleMessage(e: MessageEvent) {
      if (e.data?.received) {
        window.close();
      }
    }
    bc.addEventListener('message', handleMessage);
    bc.postMessage({ success: true });

    return () => {
      bc.removeEventListener('message', handleMessage);
      bc.close();
    };
  }, []);
  return (
    <CardSuccess title="Authorized">
      You have successfully authorized
    </CardSuccess>
  );
};
