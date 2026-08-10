import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from 'design/CollapsibleInfoSection';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { GetAccountUpgradeWindowStartHourResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Support } from 'teleport/Support';

import { SupportE } from './Support';

export default {
  title: 'TeleportE/Support',
};

interface ContactsAccess {
  list: boolean;
  create: boolean;
  remove: boolean;
  edit: boolean;
  read: boolean;
}

interface StoryContextConfig {
  hasExternalAuditStorage: boolean;
  isCloud: boolean;
  clusterId: string;
  contactsAccess: ContactsAccess;
}

const defaultContactsAccess: ContactsAccess = {
  list: true,
  create: true,
  remove: true,
  edit: true,
  read: true,
};

const readOnlyContactsAccess: ContactsAccess = {
  list: true,
  create: false,
  remove: false,
  edit: false,
  read: true,
};

function createStoryContext(config: StoryContextConfig) {
  const ctx = createTeleportContextE();
  ctx.hasExternalAuditStorage = config.hasExternalAuditStorage;
  cfg.oss.isCloud = config.isCloud;
  cfg.oss.edition = 'ent';
  cfg.oss.premiumSupport = true;
  ctx.storeUser.state.cluster.clusterId = config.clusterId;
  ctx.storeUser.getContactsAccess = () => config.contactsAccess;
  return ctx;
}

function SupportWrapper({ ctx }: { ctx: TeleportEContext }) {
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
              <Info
                kind="info"
                details="Select 16:00 and save to see error case"
              >
                Scheduled Upgrades error case
              </Info>
            </CollapsibleInfoSectionComponent>
            <Support>
              <SupportE />
            </Support>
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

const getWindowResponse: GetAccountUpgradeWindowStartHourResponse = {
  upgradeWindowStartHour: 8,
};

function createMSWHandlers(clusterId: string) {
  return [
    http.get(cfg.getContactsUrl(clusterId), () =>
      HttpResponse.json(getContactsResponse)
    ),
    http.post(cfg.getContactsUrl(clusterId), contactsPostHandler),
    http.delete(cfg.getContactsUrl(clusterId), () => HttpResponse.json(null)),
    http.get(cfg.getWindowUpgradeStartUrl('test-cluster'), () => {
      return HttpResponse.json(getWindowResponse);
    }),
    http.post(cfg.getWindowUpgradeStartUrl('test-cluster'), windowPostHandler),
    http.get(cfg.api.environmentProfileUrl, () => {
      return HttpResponse.json({ environmentProfile: 'production' });
    }),
    http.post(cfg.api.environmentProfileUrl, environmentPostHandler),
  ];
}

export const EnterpriseNonCloud = () => {
  const ctx = createStoryContext({
    hasExternalAuditStorage: false,
    isCloud: false,
    clusterId: 'test-cluster',
    contactsAccess: defaultContactsAccess,
  });

  return <SupportWrapper ctx={ctx} />;
};

EnterpriseNonCloud.beforeEach = ({ msw }) => {
  msw.use(...createMSWHandlers('test-cluster'));
};

export const CloudWithExternalAuditStorageCTA = () => {
  const ctx = createStoryContext({
    hasExternalAuditStorage: false,
    isCloud: true,
    clusterId: 'test-cluster',
    contactsAccess: defaultContactsAccess,
  });

  return <SupportWrapper ctx={ctx} />;
};

CloudWithExternalAuditStorageCTA.beforeEach = ({ msw }) => {
  msw.use(...createMSWHandlers('test-cluster'));
};

export const CloudWithoutExternalAuditStorageCTA = () => {
  const ctx = createStoryContext({
    hasExternalAuditStorage: true,
    isCloud: true,
    clusterId: 'test-cluster',
    contactsAccess: defaultContactsAccess,
  });

  return <SupportWrapper ctx={ctx} />;
};

CloudWithoutExternalAuditStorageCTA.beforeEach = ({ msw }) => {
  msw.use(...createMSWHandlers('test-cluster'));
};

export const WithContactsReadOnlyAccess = () => {
  const ctx = createStoryContext({
    hasExternalAuditStorage: true,
    isCloud: true,
    clusterId: 'test-cluster',
    contactsAccess: readOnlyContactsAccess,
  });

  return <SupportWrapper ctx={ctx} />;
};

WithContactsReadOnlyAccess.beforeEach = ({ msw }) => {
  msw.use(...createMSWHandlers('test-cluster'));
};

export const ContactsLoading = () => {
  const ctx = createStoryContext({
    hasExternalAuditStorage: true,
    isCloud: true,
    clusterId: 'test-cluster',
    contactsAccess: defaultContactsAccess,
  });

  return <SupportWrapper ctx={ctx} />;
};

ContactsLoading.beforeEach = ({ msw }) => {
  msw.use(
    http.get(
      cfg.getContactsUrl('test-cluster'),
      async () => await delay('infinite')
    )
  );
};

export const FailedContactsAndWindow = () => {
  const ctx = createStoryContext({
    hasExternalAuditStorage: true,
    isCloud: true,
    clusterId: 'test-cluster',
    contactsAccess: defaultContactsAccess,
  });

  return <SupportWrapper ctx={ctx} />;
};

FailedContactsAndWindow.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getContactsUrl('test-cluster'), () => {
      return HttpResponse.json(
        { message: 'something went wrong' },
        {
          status: 500,
        }
      );
    }),
    http.get(cfg.getWindowUpgradeStartUrl('test-cluster'), () => {
      return HttpResponse.json(
        { message: 'something went wrong' },
        {
          status: 500,
        }
      );
    })
  );
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

async function environmentPostHandler(req) {
  const { environmentProfile } = (await req.request.json()) as any;

  return HttpResponse.json({
    environmentProfile: environmentProfile,
  });
}

async function windowPostHandler(req) {
  const { upgradeWindowStartHour } = (await req.request.json()) as any;

  if (upgradeWindowStartHour === 16) {
    return HttpResponse.json(
      { error: 'Internal Server Error' },
      { status: 500, statusText: 'Internal Server Error' }
    );
  }

  return HttpResponse.json({
    upgradeWindowStartHour: upgradeWindowStartHour,
  });
}

async function contactsPostHandler(req) {
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
