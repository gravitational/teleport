import {
  Account,
  Invoice,
  BillingCycle,
  UpdateAccountRequest,
  BillingInformation,
} from 'e-teleport/services/cloud';

export const account: Account = {
  balance: -33871,
  contactEmail: 'wewko@zudhej.sy',
  contactName: 'Lucile Mann',
  companyName: 'MyCompany',
  companyAddressCity: 'Lesreur',
  companyAddressCountry: 'MD',
  companyAddressLine1: '380 Puvgi Manor',
  companyAddressLine2: '',
  companyAddressPostalCode: '323',
  companyAddressState: 'MD',
};

export const billingInformation: BillingInformation = {
  account,
  defaultPaymentMethodId: '0',
  cardsList: [
    {
      id: '0',
      last4: '4242',
      addressLine1: '',
      addressLine2: '',
      city: '',
      country: 'US',
      state: '',
      name: '',
      zip: '11111',
      brand: 'visa',
      expirationMonth: 4,
      expirationYear: 2029,
      createdAt: 1615484534,
    },
    {
      id: '1',
      last4: '4242',
      addressLine1: '',
      addressLine2: '',
      city: '',
      country: 'US',
      state: '',
      name: '',
      zip: '11111',
      brand: 'visa',
      expirationMonth: 11,
      expirationYear: 2030,
      createdAt: 1615487454,
    },
  ],
  bankAccountsList: null,
  stripePublicKey: '',
  productName: 'Teleport Pro',
};

export const invoices: Invoice[] = [
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

export const cycles: BillingCycle[] = [
  {
    cycleId: 57,
    state: 'started',
    totalAmount: 25000,
    periodStart: 1617235200, // Wed Mar 31 2021
    periodEnd: 1619827200, // Fri Apr 30 2021
    itemsList: [
      {
        cycleItemId: 'ed632646-7266-4803-8a07-d9e82f001d2c',
        quantity: 5,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'database',
        planDescription: 'Database Access',
      },
      {
        cycleItemId: '3ef33705-98da-47ec-9d1e-585d49d9171d',
        quantity: 0,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'server',
        planDescription: 'Server Access',
      },
      {
        cycleItemId: 'fcced096-2584-46d7-8dbd-97eb454bef8f',
        quantity: 34,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'application',
        planDescription: 'Application Access',
      },
      {
        cycleItemId: 'b9aedafe-9e87-415b-9476-ce44119297b2',
        quantity: 1,
        amount: 23000,
        planScheme: 'total_min',
        planResourceKind: 'other',
        planDescription: 'Additional Amount to Minimum',
      },
    ],
  },
  {
    cycleId: 0,
    state: 'ended',
    totalAmount: 0,
    periodStart: 1612165093, // jan 31 2021
    periodEnd: 1614584293, // feb 28 2021
    itemsList: [
      {
        cycleItemId: '54e97268-7a41-45b5-9395-88db34eee37f',
        quantity: 0,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'database',
        planDescription: 'Database Access',
      },
      {
        cycleItemId: '9b864173-17fb-4660-a6bc-2144325d10a6',
        quantity: 0,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'server',
        planDescription: 'Server Access',
      },
      {
        cycleItemId: 'ef8233ad-2d36-46c0-bf99-b77de75f337e',
        quantity: 0,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'application',
        planDescription: 'Application Access',
      },
      {
        cycleItemId: '861f1c68-4bfd-45d5-a7df-7cab105ce6f6',
        quantity: 0,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'kube_cluster',
        planDescription: 'Kubernetes Access',
      },
      {
        cycleItemId: '5fcc08af-8ed8-4576-b99f-fa59ac8e5400',
        quantity: 0,
        amount: 0,
        planScheme: 'total_min',
        planResourceKind: 'other',
        planDescription: 'Additional Amount to Minimum',
      },
      {
        cycleItemId: 'ceb16002-5271-4bc8-ba4a-893f504de643',
        quantity: 0,
        amount: 0,
        planScheme: '',
        planResourceKind: '',
        planDescription: 'Pro Rata Adjustment (incomplete month 36% off)',
      },
    ],
  },
  {
    cycleId: 0,
    state: 'ended',
    totalAmount: 16094,
    periodStart: 1609486693, // dec 31 2020
    periodEnd: 1612165093, // jan 31 2021,
    itemsList: [
      {
        cycleItemId: '54e97268-7a41-45b5-9395-88db34eee37f',
        quantity: 1,
        amount: 5000,
        planScheme: 'tiered',
        planResourceKind: 'database',
        planDescription: 'Database Access',
      },
      {
        cycleItemId: '9b864173-17fb-4660-a6bc-2144325d10a6',
        quantity: 5,
        amount: 499999,
        planScheme: 'tiered',
        planResourceKind: 'server',
        planDescription: 'Server Access',
      },
      {
        cycleItemId: 'ef8233ad-2d36-46c0-bf99-b77de75f337e',
        quantity: 0,
        amount: 0,
        planScheme: 'tiered',
        planResourceKind: 'application',
        planDescription: 'Application Access',
      },
      {
        cycleItemId: '861f1c68-4bfd-45d5-a7df-7cab105ce6f6',
        quantity: 350,
        amount: 25555,
        planScheme: 'tiered',
        planResourceKind: 'kube_cluster',
        planDescription: 'Kubernetes Access',
      },
      {
        cycleItemId: '5fcc08af-8ed8-4576-b99f-fa59ac8e5400',
        quantity: 0,
        amount: 0,
        planScheme: 'total_min',
        planResourceKind: 'other',
        planDescription: 'Additional Amount to Minimum',
      },
      {
        cycleItemId: 'ceb16002-5271-4bc8-ba4a-893f504de643',
        quantity: 0,
        amount: 0,
        planScheme: '',
        planResourceKind: '',
        planDescription: 'Pro Rata Adjustment (incomplete month 36% off)',
      },
    ],
  },
];

export class MockedCloudService {
  updateAccount = (acc: UpdateAccountRequest) => Promise.resolve(acc);
  fetchInvoices = () => Promise.resolve(invoices);
  fetchBillingCycles = () => Promise.resolve(cycles);
  addCard = () => Promise.resolve();
  removeCard = () => Promise.resolve();
  updateCard = () => Promise.resolve();
  fetchBillingInformation = () => Promise.resolve(billingInformation);
}
