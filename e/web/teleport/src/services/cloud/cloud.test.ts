import api from 'teleport/services/api';

import CloudSvc from './cloud';
import { Invoice, BillingInformation, BillingCycle } from './types';

test('fetch billing information', async () => {
  const cloud = new CloudSvc();
  const billingInfo = getBillingInfo();
  jest.spyOn(api, 'get').mockResolvedValue(billingInfo);

  // Test normal response.
  let response = await cloud.fetchBillingInformation();
  expect(response).toEqual(billingInfo);

  // Test with null arrays
  billingInfo.bankAccountsList = null;
  billingInfo.cardsList = null;

  response = await cloud.fetchBillingInformation();
  expect(response.bankAccountsList).toEqual([]);
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

const getBillingInfo = () =>
  ({
    account: {
      balance: -33871,
      contactEmail: 'wewko@zudhej.sy',
      contactName: 'Lucile Mann',
      companyName: 'MyCompany',
      companyAddressCity: 'Lesreur',
      companyAddressCountry: 'MD',
      companyAddressLine1: '380 Puvgi Manor',
      companyAddressLine2: 'test',
      companyAddressPostalCode: '323',
      companyAddressState: 'MD',
    },
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
    bankAccountsList: [
      {
        id: '5ff3fd22-72e8-5ce3-8c05-fe29d5b4fdef',
        accountHolderName: 'Alice',
        accountHolderType: 'test',
        bankName: 'becu',
        country: 'test',
        currency: 'test',
        customer: 'test',
        fingerprint: 'test',
        last4: '1234',
        metadata: 'test',
        routingNumber: 'test',
        status: 'test',
      },
    ],
    stripePublicKey: 'test',
    productName: 'Teleport Pro',
  } as BillingInformation);

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
