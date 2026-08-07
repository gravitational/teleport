import cfg from 'e-teleport/config';
import {
  GetStripeConfigResponse,
  GetUsageResponse,
  StripeCreateCardRequest,
  StripeCreateSetupIntentResponse,
  StripeDeleteCardRequest,
  StripeGetSettingsResponse,
  StripeListCardsResponse,
  StripeListInvoicesResponse,
  StripeUpdateCardRequest,
  StripeUpdateEmailRequest,
  StripeUpdatePOPrefixRequest,
  StripeUpdateStripeAddressRequest,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import api from 'teleport/services/api';

import CloudSvc from './cloud';

describe('cloudService', () => {
  let cloud: CloudSvc;

  beforeEach(() => {
    jest.clearAllMocks();
    cloud = new CloudSvc();
  });

  test('fetchBillingSummaryInformation', async () => {
    const expected: GetUsageResponse = {
      aggregateCount: 1,
      usageUpdatedAt: 0,
      alerts: [],
      missingEntitlements: [],
      usageHistory: [
        {
          activeAccounts: 1,
          calibratingAccounts: 0,
          end: new Date('2024/01/30').getTime(),
          endFormatted: 'Jan 30, 2024',
          pricingModel: {
            version: '4.0.0',
            modelId: '1a5451df-74e1-4332-b3b9-e397bdd29e02',
            name: 'model four',
            createdAt: 1762379155000,
            description: 'the newest pricing model',
            metric: ['MWI', 'IGMAU', 'ISTPR', 'MAU', 'TPR'],
          },
          start: new Date('2024/01/02').getTime(),
          startFormatted: 'Jan 02, 2024',
          usage: {
            ztamau: 1,
            igmau: 3,
            mwi: 33,
            tpr: 12,
          },
          usageLimits: {
            ztamau: 2,
            igmau: 4,
            mwi: 35,
            tpr: 10,
          },
        },
      ],
    };
    jest.spyOn(api, 'post').mockResolvedValue(expected);

    let response = await cloud.fetchBillingSummaryInformation({ tenants: [] });
    expect(api.post).toHaveBeenCalledWith(cfg.api.billingSummaryPath, {
      tenants: [],
    });
    expect(response).toEqual(expected);
  });

  test('getUpgradeWindowStartHour', async () => {
    jest.spyOn(api, 'get').mockResolvedValue({ upgradeWindowStartHour: 8 });

    let response = await cloud.getUpgradeWindowStartHour('clusterId');
    expect(api.get).toHaveBeenCalledWith(
      cfg.getWindowUpgradeStartUrl('clusterId')
    );
    expect(response).toEqual(8);
  });

  test('updateUpgradeWindowStart', async () => {
    jest.spyOn(api, 'post').mockResolvedValue({ upgradeWindowStartHour: 8 });

    let response = await cloud.updateUpgradeWindowStart('clusterId', 8);
    expect(api.post).toHaveBeenCalledWith(
      cfg.getWindowUpgradeStartUrl('clusterId'),
      { upgradeWindowStartHour: 8 }
    );
    expect(response).toEqual(8);
  });

  test('getEnvironmentProfile', async () => {
    jest
      .spyOn(api, 'get')
      .mockResolvedValue({ environmentProfile: 'production' });

    let response = await cloud.getEnvironmentProfile();
    expect(api.get).toHaveBeenCalledWith(cfg.api.environmentProfileUrl);
    expect(response).toEqual({ environmentProfile: 'production' });
  });

  test('updateEnvironmentProfile', async () => {
    jest
      .spyOn(api, 'post')
      .mockResolvedValue({ environmentProfile: 'production' });

    let response = await cloud.updateEnvironmentProfile('production');
    expect(api.post).toHaveBeenCalledWith(cfg.api.environmentProfileUrl, {
      environmentProfile: 'production',
    });
    expect(response).toEqual({ environmentProfile: 'production' });
  });

  test('fetchStripeConfig', async () => {
    const expected: GetStripeConfigResponse = {
      publicKey: 'pk_test_123',
      stripeCustomerId: 'cus_ABC',
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    const response = await cloud.fetchStripeConfig();
    expect(api.get).toHaveBeenCalledWith(cfg.api.stripeConfigPath);
    expect(response).toEqual(expected);
  });

  test('fetchCards', async () => {
    const expected: StripeListCardsResponse = {
      stripeCards: [
        {
          id: 'card_1',
          last4: '4242',
          addressLine1: '1 Main St',
          addressLine2: '',
          city: 'City',
          country: 'US',
          state: 'CA',
          name: 'Jane',
          zip: '94000',
          brand: 'visa',
          expirationMonth: 12,
          expirationYear: 2030,
          createdAt: 1700000000,
        },
      ],
      stripeDefaultSourceId: 'card_1',
      stripeMissingPaymentMethod: false,
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    const response = await cloud.fetchCards();
    expect(api.get).toHaveBeenCalledWith(cfg.api.stripeCardsPath);
    expect(response).toEqual(expected);
  });

  test('createSetupIntent', async () => {
    const expected: StripeCreateSetupIntentResponse = {
      clientSecret: 'seti_secret_xyz',
    };
    jest.spyOn(api, 'post').mockResolvedValue(expected);

    const response = await cloud.createSetupIntent();
    expect(api.post).toHaveBeenCalledWith(cfg.api.stripeSetupIntentPath, {});
    expect(response).toEqual(expected);
  });

  test('createCard', async () => {
    const req: StripeCreateCardRequest = {
      cardId: 'pm_123',
      isDefault: true,
    };
    jest.spyOn(api, 'post').mockResolvedValue({});

    const response = await cloud.createCard(req);
    expect(api.post).toHaveBeenCalledWith(cfg.api.stripeCardsPath, req);
    expect(response).toEqual({});
  });

  test('updateCard', async () => {
    const req: StripeUpdateCardRequest = {
      prevCardId: 'pm_old',
      nextCardId: 'pm_new',
      isDefault: true,
    };
    jest.spyOn(api, 'put').mockResolvedValue({});

    const response = await cloud.updateCard(req);
    expect(api.put).toHaveBeenCalledWith(cfg.api.stripeCardsPath, req);
    expect(response).toEqual({});
  });

  test('deleteCard', async () => {
    const req: StripeDeleteCardRequest = { cardId: 'pm_123' };
    jest.spyOn(api, 'deleteWithOptions').mockResolvedValue({});

    const response = await cloud.deleteCard(req);
    expect(api.deleteWithOptions).toHaveBeenCalledWith(
      cfg.api.stripeCardsPath,
      {
        data: req,
        headers: { 'Content-Type': 'application/json' },
      }
    );
    expect(response).toEqual({});
  });

  test('fetchInvoices', async () => {
    const expected: StripeListInvoicesResponse = {
      stripeInvoices: [
        {
          invoiceId: 'INV-1',
          status: 'paid',
          amountDue: 1000,
          amountPaid: 1000,
          periodEnd: 1700000000,
          periodStart: 1690000000,
          invoicePdf: 'https://example.com/inv.pdf',
        },
      ],
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    const response = await cloud.fetchInvoices();
    expect(api.get).toHaveBeenCalledWith(cfg.api.stripeInvoicesPath);
    expect(response).toEqual(expected);
  });

  test('fetchStripeSettings', async () => {
    const expected: StripeGetSettingsResponse = {
      stripeInvoiceBillingAddress: {
        addressCity: 'City',
        addressCountry: 'US',
        addressLine1: '1 Main St',
        addressLine2: '',
        addressPostalCode: '94000',
        addressState: 'CA',
      },
      stripeInvoiceEmail: 'billing@example.com',
      stripeInvoicePrefix: 'ACME',
      stripeCustomerName: 'Acme Inc',
      stripeSubscriptionStatus: 'active',
      planName: 'Team',
      stripeTrialEnd: 1700000000,
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    const response = await cloud.fetchStripeSettings();
    expect(api.get).toHaveBeenCalledWith(cfg.api.stripeInvoiceSettingsPath);
    expect(response).toEqual(expected);
  });

  test('fetchStripeSettings omits billing address when missing', async () => {
    jest.spyOn(api, 'get').mockResolvedValue({
      stripeInvoiceEmail: 'billing@example.com',
      stripeInvoicePrefix: '',
      stripeCustomerName: '',
      stripeSubscriptionStatus: '',
      planName: '',
      stripeTrialEnd: 0,
    });

    const response = await cloud.fetchStripeSettings();
    expect(response.stripeInvoiceBillingAddress).toBeUndefined();
    expect(response.stripeInvoiceEmail).toEqual('billing@example.com');
  });

  test('updateInvoiceEmail', async () => {
    const req: StripeUpdateEmailRequest = { email: 'billing@example.com' };
    jest.spyOn(api, 'post').mockResolvedValue({});

    const response = await cloud.updateInvoiceEmail(req);
    expect(api.post).toHaveBeenCalledWith(cfg.api.stripeInvoiceEmailPath, req);
    expect(response).toEqual({});
  });

  test('updatePurchaseOrderPrefix', async () => {
    const req: StripeUpdatePOPrefixRequest = { po: 'ACME' };
    jest.spyOn(api, 'post').mockResolvedValue({});

    const response = await cloud.updatePurchaseOrderPrefix(req);
    expect(api.post).toHaveBeenCalledWith(
      cfg.api.stripeInvoicePOPrefixPath,
      req
    );
    expect(response).toEqual({});
  });

  test('updateStripeAddress', async () => {
    const req: StripeUpdateStripeAddressRequest = {
      name: 'Acme Inc',
      address: {
        addressCity: 'City',
        addressCountry: 'US',
        addressLine1: '1 Main St',
        addressLine2: '',
        addressPostalCode: '94000',
        addressState: 'CA',
      },
    };
    jest.spyOn(api, 'post').mockResolvedValue({});

    const response = await cloud.updateStripeAddress(req);
    expect(api.post).toHaveBeenCalledWith(
      cfg.api.stripeInvoiceAddressPath,
      req
    );
    expect(response).toEqual({});
  });

  test('cancelSubscription', async () => {
    jest.spyOn(api, 'post').mockResolvedValue({});

    const response = await cloud.cancelSubscription();
    expect(api.post).toHaveBeenCalledWith(cfg.api.stripeCancelPath, {});
    expect(response).toEqual({});
  });
});
