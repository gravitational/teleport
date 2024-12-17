import { MemoryRouter } from 'react-router-dom';

import { RequestList } from './RequestList';
import { sample } from './fixtures';

export default {
  title: 'TeleportE/AccessRequests/RequestList',
};

export const Processing = () => {
  return (
    <MemoryRouter>
      <RequestList {...sample} attempt={{ status: 'processing' }} />
    </MemoryRouter>
  );
};

export const Loaded = () => {
  return (
    <MemoryRouter>
      <RequestList {...sample} />
    </MemoryRouter>
  );
};

export const Failed = () => {
  return (
    <MemoryRouter>
      <RequestList
        {...sample}
        attempt={{ status: 'failed', statusText: 'some error message' }}
      />
    </MemoryRouter>
  );
};
