// package: gravitational.cloud.tenants.v1
// file: api/tenants/v1/tenants.proto

/* eslint-disable */

import * as jspb from 'google-protobuf';

export class Account extends jspb.Message {
  getContactEmail(): string;
  setContactEmail(value: string): void;

  getContactName(): string;
  setContactName(value: string): void;

  getCompanyName(): string;
  setCompanyName(value: string): void;

  getCompanyAddressCity(): string;
  setCompanyAddressCity(value: string): void;

  getCompanyAddressCountry(): string;
  setCompanyAddressCountry(value: string): void;

  getCompanyAddressLine1(): string;
  setCompanyAddressLine1(value: string): void;

  getCompanyAddressLine2(): string;
  setCompanyAddressLine2(value: string): void;

  getCompanyAddressPostalCode(): string;
  setCompanyAddressPostalCode(value: string): void;

  getCompanyAddressState(): string;
  setCompanyAddressState(value: string): void;

  getBalance(): number;
  setBalance(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Account.AsObject;
  static toObject(includeInstance: boolean, msg: Account): Account.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: Account,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): Account;
  static deserializeBinaryFromReader(
    message: Account,
    reader: jspb.BinaryReader
  ): Account;
}

export namespace Account {
  export type AsObject = {
    contactEmail: string;
    contactName: string;
    companyName: string;
    companyAddressCity: string;
    companyAddressCountry: string;
    companyAddressLine1: string;
    companyAddressLine2: string;
    companyAddressPostalCode: string;
    companyAddressState: string;
    balance: number;
  };
}

export class Card extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  getLast4(): string;
  setLast4(value: string): void;

  getAddressLine1(): string;
  setAddressLine1(value: string): void;

  getAddressLine2(): string;
  setAddressLine2(value: string): void;

  getCity(): string;
  setCity(value: string): void;

  getCountry(): string;
  setCountry(value: string): void;

  getState(): string;
  setState(value: string): void;

  getName(): string;
  setName(value: string): void;

  getZip(): string;
  setZip(value: string): void;

  getBrand(): string;
  setBrand(value: string): void;

  getExpirationMonth(): number;
  setExpirationMonth(value: number): void;

  getExpirationYear(): number;
  setExpirationYear(value: number): void;

  getCreatedAt(): number;
  setCreatedAt(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Card.AsObject;
  static toObject(includeInstance: boolean, msg: Card): Card.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: Card,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): Card;
  static deserializeBinaryFromReader(
    message: Card,
    reader: jspb.BinaryReader
  ): Card;
}

export namespace Card {
  export type AsObject = {
    id: string;
    last4: string;
    addressLine1: string;
    addressLine2: string;
    city: string;
    country: string;
    state: string;
    name: string;
    zip: string;
    brand: string;
    expirationMonth: number;
    expirationYear: number;
    createdAt: number;
  };
}

export class BankAccount extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  getAccountHolderName(): string;
  setAccountHolderName(value: string): void;

  getAccountHolderType(): string;
  setAccountHolderType(value: string): void;

  getBankName(): string;
  setBankName(value: string): void;

  getCountry(): string;
  setCountry(value: string): void;

  getCurrency(): string;
  setCurrency(value: string): void;

  getCustomer(): string;
  setCustomer(value: string): void;

  getFingerprint(): string;
  setFingerprint(value: string): void;

  getLast4(): string;
  setLast4(value: string): void;

  getMetadata(): string;
  setMetadata(value: string): void;

  getRoutingNumber(): string;
  setRoutingNumber(value: string): void;

  getStatus(): string;
  setStatus(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BankAccount.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: BankAccount
  ): BankAccount.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: BankAccount,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): BankAccount;
  static deserializeBinaryFromReader(
    message: BankAccount,
    reader: jspb.BinaryReader
  ): BankAccount;
}

export namespace BankAccount {
  export type AsObject = {
    id: string;
    accountHolderName: string;
    accountHolderType: string;
    bankName: string;
    country: string;
    currency: string;
    customer: string;
    fingerprint: string;
    last4: string;
    metadata: string;
    routingNumber: string;
    status: string;
  };
}

export class GetBillingInformationResponse extends jspb.Message {
  hasAccount(): boolean;
  clearAccount(): void;
  getAccount(): Account | undefined;
  setAccount(value?: Account): void;

  getDefaultPaymentMethodId(): string;
  setDefaultPaymentMethodId(value: string): void;

  clearCardsList(): void;
  getCardsList(): Array<Card>;
  setCardsList(value: Array<Card>): void;
  addCards(value?: Card, index?: number): Card;

  clearBankAccountsList(): void;
  getBankAccountsList(): Array<BankAccount>;
  setBankAccountsList(value: Array<BankAccount>): void;
  addBankAccounts(value?: BankAccount, index?: number): BankAccount;

  getStripePublicKey(): string;
  setStripePublicKey(value: string): void;

  getProductName(): string;
  setProductName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBillingInformationResponse.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: GetBillingInformationResponse
  ): GetBillingInformationResponse.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: GetBillingInformationResponse,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): GetBillingInformationResponse;
  static deserializeBinaryFromReader(
    message: GetBillingInformationResponse,
    reader: jspb.BinaryReader
  ): GetBillingInformationResponse;
}

export namespace GetBillingInformationResponse {
  export type AsObject = {
    account?: Account.AsObject;
    defaultPaymentMethodId: string;
    cardsList: Array<Card.AsObject>;
    bankAccountsList: Array<BankAccount.AsObject>;
    stripePublicKey: string;
    productName: string;
  };
}

export class ListInvoicesResponse extends jspb.Message {
  clearInvoicesList(): void;
  getInvoicesList(): Array<Invoice>;
  setInvoicesList(value: Array<Invoice>): void;
  addInvoices(value?: Invoice, index?: number): Invoice;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListInvoicesResponse.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: ListInvoicesResponse
  ): ListInvoicesResponse.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: ListInvoicesResponse,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): ListInvoicesResponse;
  static deserializeBinaryFromReader(
    message: ListInvoicesResponse,
    reader: jspb.BinaryReader
  ): ListInvoicesResponse;
}

export namespace ListInvoicesResponse {
  export type AsObject = {
    invoicesList: Array<Invoice.AsObject>;
  };
}

export class Invoice extends jspb.Message {
  getInvoiceId(): string;
  setInvoiceId(value: string): void;

  getStatus(): string;
  setStatus(value: string): void;

  getAmountDue(): number;
  setAmountDue(value: number): void;

  getAmountPaid(): number;
  setAmountPaid(value: number): void;

  getPeriodEnd(): number;
  setPeriodEnd(value: number): void;

  getPeriodStart(): number;
  setPeriodStart(value: number): void;

  getInvoicePdf(): string;
  setInvoicePdf(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Invoice.AsObject;
  static toObject(includeInstance: boolean, msg: Invoice): Invoice.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: Invoice,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): Invoice;
  static deserializeBinaryFromReader(
    message: Invoice,
    reader: jspb.BinaryReader
  ): Invoice;
}

export namespace Invoice {
  export type AsObject = {
    invoiceId: string;
    status: string;
    amountDue: number;
    amountPaid: number;
    periodEnd: number;
    periodStart: number;
    invoicePdf: string;
  };
}

export class BillingCycle extends jspb.Message {
  getCycleId(): number;
  setCycleId(value: number): void;

  getState(): string;
  setState(value: string): void;

  getTotalAmount(): number;
  setTotalAmount(value: number): void;

  getPeriodStart(): number;
  setPeriodStart(value: number): void;

  getPeriodEnd(): number;
  setPeriodEnd(value: number): void;

  clearItemsList(): void;
  getItemsList(): Array<BillingCycleItem>;
  setItemsList(value: Array<BillingCycleItem>): void;
  addItems(value?: BillingCycleItem, index?: number): BillingCycleItem;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BillingCycle.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: BillingCycle
  ): BillingCycle.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: BillingCycle,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): BillingCycle;
  static deserializeBinaryFromReader(
    message: BillingCycle,
    reader: jspb.BinaryReader
  ): BillingCycle;
}

export namespace BillingCycle {
  export type AsObject = {
    cycleId: number;
    state: string;
    totalAmount: number;
    periodStart: number;
    periodEnd: number;
    itemsList: Array<BillingCycleItem.AsObject>;
  };
}

export class BillingCycleItem extends jspb.Message {
  getCycleItemId(): string;
  setCycleItemId(value: string): void;

  getQuantity(): number;
  setQuantity(value: number): void;

  getAmount(): number;
  setAmount(value: number): void;

  getPlanScheme(): string;
  setPlanScheme(value: string): void;

  getPlanResourceKind(): string;
  setPlanResourceKind(value: string): void;

  getPlanDescription(): string;
  setPlanDescription(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BillingCycleItem.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: BillingCycleItem
  ): BillingCycleItem.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: BillingCycleItem,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): BillingCycleItem;
  static deserializeBinaryFromReader(
    message: BillingCycleItem,
    reader: jspb.BinaryReader
  ): BillingCycleItem;
}

export namespace BillingCycleItem {
  export type AsObject = {
    cycleItemId: string;
    quantity: number;
    amount: number;
    planScheme: string;
    planResourceKind: string;
    planDescription: string;
  };
}

export class ListBillingCyclesResponse extends jspb.Message {
  clearCyclesList(): void;
  getCyclesList(): Array<BillingCycle>;
  setCyclesList(value: Array<BillingCycle>): void;
  addCycles(value?: BillingCycle, index?: number): BillingCycle;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListBillingCyclesResponse.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: ListBillingCyclesResponse
  ): ListBillingCyclesResponse.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: ListBillingCyclesResponse,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): ListBillingCyclesResponse;
  static deserializeBinaryFromReader(
    message: ListBillingCyclesResponse,
    reader: jspb.BinaryReader
  ): ListBillingCyclesResponse;
}

export namespace ListBillingCyclesResponse {
  export type AsObject = {
    cyclesList: Array<BillingCycle.AsObject>;
  };
}

export class SubmitUsageReportsRequest extends jspb.Message {
  clearReportsList(): void;
  getReportsList(): Array<UsageReport>;
  setReportsList(value: Array<UsageReport>): void;
  addReports(value?: UsageReport, index?: number): UsageReport;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubmitUsageReportsRequest.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: SubmitUsageReportsRequest
  ): SubmitUsageReportsRequest.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: SubmitUsageReportsRequest,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): SubmitUsageReportsRequest;
  static deserializeBinaryFromReader(
    message: SubmitUsageReportsRequest,
    reader: jspb.BinaryReader
  ): SubmitUsageReportsRequest;
}

export namespace SubmitUsageReportsRequest {
  export type AsObject = {
    reportsList: Array<UsageReport.AsObject>;
  };
}

export class UsageReport extends jspb.Message {
  getPeriodstart(): number;
  setPeriodstart(value: number): void;

  getPeriodend(): number;
  setPeriodend(value: number): void;

  clearItemsList(): void;
  getItemsList(): Array<UsageReportItem>;
  setItemsList(value: Array<UsageReportItem>): void;
  addItems(value?: UsageReportItem, index?: number): UsageReportItem;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UsageReport.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: UsageReport
  ): UsageReport.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: UsageReport,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): UsageReport;
  static deserializeBinaryFromReader(
    message: UsageReport,
    reader: jspb.BinaryReader
  ): UsageReport;
}

export namespace UsageReport {
  export type AsObject = {
    periodstart: number;
    periodend: number;
    itemsList: Array<UsageReportItem.AsObject>;
  };
}

export class UsageReportItem extends jspb.Message {
  getResource(): UsageResourceTypeMap[keyof UsageResourceTypeMap];
  setResource(value: UsageResourceTypeMap[keyof UsageResourceTypeMap]): void;

  getQuantity(): number;
  setQuantity(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UsageReportItem.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: UsageReportItem
  ): UsageReportItem.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: UsageReportItem,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): UsageReportItem;
  static deserializeBinaryFromReader(
    message: UsageReportItem,
    reader: jspb.BinaryReader
  ): UsageReportItem;
}

export namespace UsageReportItem {
  export type AsObject = {
    resource: UsageResourceTypeMap[keyof UsageResourceTypeMap];
    quantity: number;
  };
}

export class RemoveCardRequest extends jspb.Message {
  getCardId(): string;
  setCardId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RemoveCardRequest.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: RemoveCardRequest
  ): RemoveCardRequest.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: RemoveCardRequest,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): RemoveCardRequest;
  static deserializeBinaryFromReader(
    message: RemoveCardRequest,
    reader: jspb.BinaryReader
  ): RemoveCardRequest;
}

export namespace RemoveCardRequest {
  export type AsObject = {
    cardId: string;
  };
}

export class AddCardRequest extends jspb.Message {
  getCardId(): string;
  setCardId(value: string): void;

  getIsDefault(): boolean;
  setIsDefault(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AddCardRequest.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: AddCardRequest
  ): AddCardRequest.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: AddCardRequest,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): AddCardRequest;
  static deserializeBinaryFromReader(
    message: AddCardRequest,
    reader: jspb.BinaryReader
  ): AddCardRequest;
}

export namespace AddCardRequest {
  export type AsObject = {
    cardId: string;
    isDefault: boolean;
  };
}

export class UpdateCardRequest extends jspb.Message {
  getPrevCardId(): string;
  setPrevCardId(value: string): void;

  getNextCardId(): string;
  setNextCardId(value: string): void;

  getIsDefault(): boolean;
  setIsDefault(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateCardRequest.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: UpdateCardRequest
  ): UpdateCardRequest.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: UpdateCardRequest,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): UpdateCardRequest;
  static deserializeBinaryFromReader(
    message: UpdateCardRequest,
    reader: jspb.BinaryReader
  ): UpdateCardRequest;
}

export namespace UpdateCardRequest {
  export type AsObject = {
    prevCardId: string;
    nextCardId: string;
    isDefault: boolean;
  };
}

export class UpdateAccountRequest extends jspb.Message {
  hasAccount(): boolean;
  clearAccount(): void;
  getAccount(): Account | undefined;
  setAccount(value?: Account): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateAccountRequest.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: UpdateAccountRequest
  ): UpdateAccountRequest.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: UpdateAccountRequest,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): UpdateAccountRequest;
  static deserializeBinaryFromReader(
    message: UpdateAccountRequest,
    reader: jspb.BinaryReader
  ): UpdateAccountRequest;
}

export namespace UpdateAccountRequest {
  export type AsObject = {
    account?: Account.AsObject;
  };
}

export class EmptyResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EmptyResponse.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: EmptyResponse
  ): EmptyResponse.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: EmptyResponse,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): EmptyResponse;
  static deserializeBinaryFromReader(
    message: EmptyResponse,
    reader: jspb.BinaryReader
  ): EmptyResponse;
}

export namespace EmptyResponse {
  export type AsObject = {};
}

export class EmptyRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EmptyRequest.AsObject;
  static toObject(
    includeInstance: boolean,
    msg: EmptyRequest
  ): EmptyRequest.AsObject;
  static extensions: { [key: number]: jspb.ExtensionFieldInfo<jspb.Message> };
  static extensionsBinary: {
    [key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>;
  };
  static serializeBinaryToWriter(
    message: EmptyRequest,
    writer: jspb.BinaryWriter
  ): void;
  static deserializeBinary(bytes: Uint8Array): EmptyRequest;
  static deserializeBinaryFromReader(
    message: EmptyRequest,
    reader: jspb.BinaryReader
  ): EmptyRequest;
}

export namespace EmptyRequest {
  export type AsObject = {};
}

export interface UsageResourceTypeMap {
  UNKNOWN: 0;
  USER: 1;
  SERVER: 2;
  KUBE_CLUSTER: 3;
  DATABASE: 4;
  APPLICATION: 5;
  ROLE: 6;
  AUTH_CONNECTOR: 7;
}

export const UsageResourceType: UsageResourceTypeMap;
