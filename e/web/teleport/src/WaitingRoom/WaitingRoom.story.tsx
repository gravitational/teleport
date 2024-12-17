import { WaitingRoomComponent as WaitingRoom } from './WaitingRoom';
import RequestPending from './RequestPending';

export default {
  title: 'TeleportE/WaitingRoom',
};

export const Processing = () => {
  const attempt = {
    isProcessing: true,
    isFailed: false,
    isSuccess: false,
    message: '',
  };
  return <WaitingRoom {...sample} attempt={attempt} />;
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error',
  };
  return <WaitingRoom {...sample} attempt={attempt} />;
};

export const Pending = () => {
  return <RequestPending />;
};

const sample = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: false,
    message: '',
  },
  strategy: {
    type: 'optional' as any,
    prompt: '',
  },
  accessRequest: null,
  createRequest: null,
  refresh: null,
  children: null,
  checkerInterval: 0,
};
