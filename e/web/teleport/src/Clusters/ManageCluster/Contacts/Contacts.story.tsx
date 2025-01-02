import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { Route } from 'teleport/components/Router';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';

import { Contacts } from './Contacts';

export default {
  title: 'TeleportE/Clusters/Contacts',
};

function render({ read, write }: { read: boolean; write: boolean }) {
  const ctx = createTeleportContextE();

  ctx.storeUser.getContactsAccess = () => ({
    list: read,
    create: write,
    remove: write,
    edit: write,
    read: read,
  });

  return (
    <MemoryRouter initialEntries={['/clusters/test-cluster']}>
      <Route path="/clusters/:clusterId">
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>
            <Contacts />
          </ContextProvider>
        </ContentMinWidth>
      </Route>
    </MemoryRouter>
  );
}

export function Loading() {
  return render({ read: true, write: true });
}

Loading.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getContactsUrl('test-cluster'),
        async () => await delay('infinite')
      ),
    ],
  },
};

export function Loaded() {
  return render({ read: true, write: true });
}

Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getContactsUrl('test-cluster'), () =>
        HttpResponse.json(getContactsResponse)
      ),
      http.post(cfg.getContactsUrl('test-cluster'), postHandler),
      http.delete(cfg.getContactsUrl('test-cluster'), () =>
        HttpResponse.json(null)
      ),
    ],
  },
};

export function ReadAccessOnly() {
  return render({ read: true, write: false });
}

ReadAccessOnly.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getContactsUrl('test-cluster'), () =>
        HttpResponse.json(getContactsResponse)
      ),
    ],
  },
};

export function ErrorLoading() {
  return render({ read: true, write: true });
}

ErrorLoading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getContactsUrl('test-cluster'), () => {
        return HttpResponse.json(
          { message: 'something went wrong' },
          {
            status: 500,
          }
        );
      }),
    ],
  },
};

const getContactsResponse = {
  contacts: [
    {
      accountID: 'a242b735-fee2-4f66-afa5-3774932efccf',
      verifyToken: '648351f4-0db8-4e90-a313-75d176120367',
      email: 'contact1@goteleport.com',
      contactType: 1,
      verified: true,
      verifyExpiresAt: '1734000281',
      contactState: 'CONTACT_STATE_ACTIVE',
    },
    {
      name: '',
      accountID: 'a242b735-fee2-4f66-afa5-3774932efccf',
      verifyToken: '5582c8b9-aa74-40ce-ab95-18d7b0e20961',
      email: 'contact2@gmail.com',
      contactType: 2,
      verified: true,
      contactState: 'CONTACT_STATE_ACTIVE',
    },
    {
      name: '',
      accountID: 'a242b735-fee2-4f66-afa5-3774932efccf',
      verifyToken: '5582c8b9-aa74-40ce-ab95-18d7b0e20962',
      email: 'contact3@gmail.com',
      contactType: 2,
      verified: true,
      contactState: 'CONTACT_STATE_ACTIVE',
    },
    {
      name: '',
      accountID: 'a242b735-fee2-4f66-afa5-3774932efccf',
      verifyToken: '5582c8b9-aa74-40ce-ab95-18d7b0e20963',
      email: 'contact4@gmail.com',
      contactType: 2,
      verified: true,
      contactState: 'CONTACT_STATE_PENDING',
    },
  ],
};

async function postHandler(req) {
  const { email, contact_type } = (await req.request.json()) as any;

  return HttpResponse.json({
    accountID: 'a242b735-fee2-4f66-afa5-3774932efccf',
    verifyToken: 'new-verify-token' + new Date().toString,
    email: email,
    contactType: contact_type,
    verified: false,
    verifyExpiresAt: new Date('3000-01-01').getDate(),
    contactState: 'CONTACT_STATE_PENDING',
  });
}
