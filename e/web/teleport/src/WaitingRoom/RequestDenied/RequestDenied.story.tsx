import RequestDenied from './RequestDenied';

export default {
  title: 'TeleportE/WaitingRoom/Denied',
};

export const WithReason = () => {
  return <RequestDenied {...sample} />;
};

export const WithoutReason = () => {
  return <RequestDenied {...sample} reason={''} />;
};

const sample = {
  reason: 'some reason for denying request',
};
