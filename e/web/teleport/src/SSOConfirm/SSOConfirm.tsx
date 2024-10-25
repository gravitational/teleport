import React, { useEffect } from 'react';
import { useLocation } from 'react-router';

import { CardSuccess } from 'design';
import { Failed } from 'design/CardError';

export const SSOConfirm = () => {
  const { search } = useLocation();
  const params = new URLSearchParams(search);
  const ssoResponse = params.get('response');
  const channelId = params.get('channel_id');

  useEffect(() => {
    if (!channelId || !ssoResponse) {
      return;
    }

    const bc = new BroadcastChannel(channelId);
    if (ssoResponse) {
      const data = JSON.parse(ssoResponse);
      bc.postMessage({ mfaToken: data.mfa_token });
      setTimeout(() => {
        window.close();
      }, 1000);
    }

    return () => {
      bc.close();
    };
  }, [ssoResponse, channelId]);

  // we don't do any validation here except checking the existence of the token.
  // The validation happens when the other end of the broadcast token sends it off.
  if (!ssoResponse || !channelId) {
    return <Failed message="Invalid or missing token" />;
  }

  return (
    <CardSuccess title="Authenticated">
      You have successfully authenticated.
    </CardSuccess>
  );
};
