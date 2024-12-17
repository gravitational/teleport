import { SAMLLoginAccessDenied, SAMLLoginProcessing } from './SAMLIdPLogin';

export default {
  title: 'Teleport/SAMLIdPLogin',
};

export const Processing = () => {
  return <SAMLLoginProcessing />;
};

export const Failed = () => {
  return <SAMLLoginAccessDenied statusText="" />;
};
