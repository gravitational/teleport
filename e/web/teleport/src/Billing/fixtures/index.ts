import { Account, Invoice } from 'e-teleport/services/cloud';

export const account: Account = {
  balance: 32323,
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

export class MockedBillingService {
  fetchAccount = () => Promise.resolve(account);
  updateAccount = (acc: Account) => Promise.resolve(acc);
  fetchInvoices = () => Promise.resolve(invoices);
}
