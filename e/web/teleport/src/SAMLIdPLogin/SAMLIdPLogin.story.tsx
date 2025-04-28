import cfg from 'e-teleport/config';
import history from 'teleport/services/history';

import {
  SAMLIdPLogin,
  SAMLLoginAccessDenied,
  SAMLLoginProcessing,
} from './SAMLIdPLogin';

export default {
  title: 'Teleport/SAMLIdPLogin',
};

export const Processing = () => {
  return <SAMLLoginProcessing />;
};

export const Failed = () => {
  return <SAMLLoginAccessDenied statusText="" />;
};

export const BadRequest = () => {
  history.getRedirectParam = () =>
    `https://example.com${cfg.routes.samlIdPLogin}`;
  return <SAMLIdPLogin />;
};
