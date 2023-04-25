import api from 'teleport/services/api';

import CloudSvc from './cloud';
import { BillingCycle, BillingInformation, Invoice } from './types';

test('fetch billing information', async () => {
  const cloud = new CloudSvc();
  const billingInfo = getDefaultBillingInfo();
  jest.spyOn(api, 'get').mockResolvedValue(billingInfo);

  // Test normal response.
  let response = await cloud.fetchBillingInformation();
  expect(response).toEqual(billingInfo);

  // Test with null arrays
  billingInfo.cardsList = null;

  response = await cloud.fetchBillingInformation();
  expect(response.cardsList).toEqual([]);
});

test('fetch invoices', async () => {
  const cloud = new CloudSvc();

  // Test normal response.
  jest.spyOn(api, 'get').mockResolvedValue(invoices);
  let response = await cloud.fetchInvoices();
  expect(response).toEqual(invoices);

  // Test with null resolved value
  jest.spyOn(api, 'get').mockResolvedValue(null);
  response = await cloud.fetchInvoices();
  expect(response).toEqual([]);
});

test('fetch billing cycles', async () => {
  const cloud = new CloudSvc();
  const cycles = getBillingCycleList();
  jest.spyOn(api, 'get').mockResolvedValue(cycles);

  // Test normal response.
  let response = await cloud.fetchBillingCycles();
  expect(response).toEqual(cycles);

  // Test with null arrays
  cycles[0].itemsList = null;
  response = await cloud.fetchBillingCycles();
  expect(response[0].itemsList).toEqual([]);

  // Test with null resolved value
  jest.spyOn(api, 'get').mockResolvedValue(null);
  response = await cloud.fetchBillingCycles();
  expect(response).toEqual([]);
});

const getBillingCycleList = () =>
  [
    {
      cycleId: 57,
      state: 'started',
      totalAmount: 25000,
      periodStart: 1617235200, // Wed Mar 31 2021
      periodEnd: 1619827200, // Fri Apr 30 2021
      itemsList: [
        {
          cycleItemId: 'ed632646-7266-4803-8a07-d9e82f001d2c',
          quantity: 0,
          amount: 0,
          planScheme: 'tiered',
          planResourceKind: 'database',
          planDescription: 'Database Access',
        },
      ],
    },
  ] as BillingCycle[];

const getDefaultBillingInfo = (): BillingInformation => ({
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
});

const invoices: Invoice[] = [
  {
    invoiceId: 'ca3f7381-eb82-5066-8488-86029312b7ae',
    status: 'PENDING',
    amountDue: 30000,
    amountPaid: 30000,
    periodEnd: 1617208855,
    periodStart: 1614616855,
    invoicePdf: 'direct-link-to-stripe-invoice',
  },
  {
    invoiceId: 'ca3f7381-eb82-5066-8488-86029312b7ae',
    status: 'PAID',
    amountDue: 30000,
    amountPaid: 30000,
    periodEnd: 1617208855,
    periodStart: 1614616855,
    invoicePdf: 'direct-link-to-stripe-invoice',
  },
];
