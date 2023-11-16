import api from 'teleport/services/api';

import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';

import cfg from 'e-teleport/config';

import CloudSvc from './cloud';
import {
  AddCardRequest,
  BillingInformation,
  BillingSummaryInformation,
  InvoiceSettingsInformation,
  PaymentsInvoicesInformation,
  RemoveCardRequest,
  SetupIntent,
  StripeBillingAddressRequest,
  UpdateCardRequest,
  UpdateEmailRequest,
  UpdatePurchaseOrderRequest,
} from './types';

describe('cloudService', () => {
  let cloud: CloudSvc;

  beforeEach(() => {
    jest.clearAllMocks();
    cloud = new CloudSvc();
  });

  test('updateAddress', async () => {
    jest.spyOn(api, 'put').mockResolvedValue(null);
    const req: StripeBillingAddressRequest = {
      name: 'jane doe',
      address: {
        addressCity: 'city',
        addressCountry: 'country',
        addressLine1: 'line1',
        addressLine2: 'line2',
        addressPostalCode: 'postal_code',
        addressState: 'state',
      },
    };

    await cloud.updateAddress(req);

    expect(api.put).toHaveBeenCalledWith(
      cfg.api.addressPath,
      expect.objectContaining(req)
    );
  });

  test('addCard', async () => {
    jest.spyOn(api, 'post').mockResolvedValue(null);
    const req: AddCardRequest = {
      cardId: 'some-id',
      isDefault: true,
    };

    await cloud.addCard(req);

    expect(api.post).toHaveBeenCalledWith(
      cfg.api.cardPath,
      expect.objectContaining(req)
    );
  });

  test('removeCard', async () => {
    jest.spyOn(api, 'delete').mockResolvedValue(null);
    const req: RemoveCardRequest = {
      cardId: 'some-id',
    };

    await cloud.removeCard(req);

    expect(api.delete).toHaveBeenCalledWith(
      cfg.api.cardPath,
      expect.objectContaining(req)
    );
  });

  test('updateCard', async () => {
    jest.spyOn(api, 'delete').mockResolvedValue(null);
    const req: UpdateCardRequest = {
      prevCardId: 'prev-id',
      nextCardId: 'next-id',
      isDefault: false,
    };

    await cloud.updateCard(req);

    expect(api.put).toHaveBeenCalledWith(
      cfg.api.cardPath,
      expect.objectContaining(req)
    );
  });

  test('createSetupIntent', async () => {
    const expected: SetupIntent = {
      clientSecret: 'some-secret',
    };
    jest.spyOn(api, 'post').mockResolvedValue(expected);

    let response = await cloud.createSetupIntent();
    expect(api.post).toHaveBeenCalledWith(cfg.api.setupIntentPath);
    expect(response).toEqual(expected);
  });

  test('fetch billing information', async () => {
    const cloud = new CloudSvc();
    const expected: BillingInformation = {
      defaultPaymentMethodId: '41c23fa8-5914-5283-90f7-2146bec2c39f',
      cardsList: [
        {
          id: 'abc',
          last4: '4242',
          addressLine1: '1234 W',
          addressLine2: '5678 N',
          city: 'Seattle',
          country: 'US',
          state: 'WA',
          name: 'Bob',
          zip: '11111',
          brand: 'visa',
          expirationMonth: 4,
          expirationYear: 2029,
          createdAt: 1615484534,
        },
      ],
      stripePublicKey: 'test',
      productName: 'Teleport Pro',
      trial: false,
      selfEnrolled: false,
      upsellAlert: false,
      usageBasedBilling: false,
      stripeTrial: false,
      stripeTrialEnd: 0,
      stripeMissingPaymentMethod: false,
      stripeCustomerId: '6e5357a5-e7e9-4770-9235-aab26936b5b0',
      stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,
      stripeSubscriptionCancelAt: 0,
      stripeSubscriptionCanceledAt: 0,
    };

    jest.spyOn(api, 'get').mockResolvedValue(expected);

    // Test normal response.
    let response = await cloud.fetchBillingInformation();
    expect(response).toEqual(expected);

    // Test with null arrays
    expected.cardsList = null;

    response = await cloud.fetchBillingInformation();
    expect(response.cardsList).toEqual([]);
  });

  test('fetchBillingSummaryInformation', async () => {
    const expected: BillingSummaryInformation = {
      usageBasedBilling: false,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeCurrentUsage: {
        invoiceId: 'some-invoiceId',
        status: 'some-status',
        periodEnd: 1684773766,
        periodStart: 1684773766,
        usageMau: 0,
        usageTia: 1,
        usagePr: 2,
      },
      stripeTrial: true,
      stripeTrialEnd: 1684773766,
      stripeMissingPaymentMethod: false,
      productName: 'some-productName',
      stripeSubscriptionStatus: 'some-stripeSubscriptionStatus',
      stripeSubscriptionCancelAt: 1684773766,
      stripeSubscriptionCanceledAt: 1684773766,
      usageUpdatedAt: 0,
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    let response = await cloud.fetchBillingSummaryInformation();
    expect(api.get).toHaveBeenCalledWith(cfg.api.billingSummaryPath);
    expect(response).toEqual(expected);
  });

  test('fetchPaymentsAndInvoices', async () => {
    const expected: PaymentsInvoicesInformation = {
      usageBasedBilling: true,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeMissingPaymentMethod: true,
      stripeCardsList: [],
      stripeInvoicesList: [],
      productName: 'some-productName',
      stripeTrialEnd: 0,
      stripeDefaultSourceId: 'some-stripeDefaultSourceId',
      stripeSubscriptionStatus: 'some-stripeSubscriptionStatus',
      stripeSubscriptionCancelAt: 0,
      stripeSubscriptionCanceledAt: 0,
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    let response = await cloud.fetchPaymentsAndInvoices();
    expect(api.get).toHaveBeenCalledWith(cfg.api.paymentsInvoicesPath);
    expect(response).toEqual(expected);
  });

  test('fetchInvoiceSettings', async () => {
    const expected: InvoiceSettingsInformation = {
      usageBasedBilling: false,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeInvoiceEmail: 'some-stripeInvoiceEmail',
      stripeInvoicePurchaseOrderNumber: 'some-stripeInvoicePurchaseOrderNumber',
      stripeCustomerName: 'some-stripeCustomerName',
      stripeSubscriptionStatus: 'some-stripeSubscriptionStatus',
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    let response = await cloud.fetchInvoiceSettings();
    expect(api.get).toHaveBeenCalledWith(cfg.api.invoiceSettingsPath);
    expect(response).toEqual(expected);
  });

  test('updateEmail', async () => {
    jest.spyOn(api, 'put').mockResolvedValue(null);
    const req: UpdateEmailRequest = {
      email: 'hi@example.com',
    };

    await cloud.updateEmail(req);

    expect(api.put).toHaveBeenCalledWith(
      cfg.api.emailPath,
      expect.objectContaining(req)
    );
  });

  test('updatePurchaseOrderPrefix', async () => {
    jest.spyOn(api, 'put').mockResolvedValue(null);
    const req: UpdatePurchaseOrderRequest = {
      po: '9090',
    };

    await cloud.updatePurchaseOrderPrefix(req);

    expect(api.put).toHaveBeenCalledWith(
      cfg.api.poPath,
      expect.objectContaining(req)
    );
  });

  test('cancelSubscription', async () => {
    jest.spyOn(api, 'delete').mockResolvedValue(null);

    await cloud.cancelSubscription();

    expect(api.delete).toHaveBeenCalledWith(cfg.api.billingPath);
  });
});
