// package: gravitational.cloud.tenants.v1
// file: api/tenants/v1/tenants.proto

import * as jspb from "google-protobuf";

export class StripeBillingAddressRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasAddress(): boolean;
  clearAddress(): void;
  getAddress(): StripeBillingAddress | undefined;
  setAddress(value?: StripeBillingAddress): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StripeBillingAddressRequest.AsObject;
  static toObject(includeInstance: boolean, msg: StripeBillingAddressRequest): StripeBillingAddressRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StripeBillingAddressRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StripeBillingAddressRequest;
  static deserializeBinaryFromReader(message: StripeBillingAddressRequest, reader: jspb.BinaryReader): StripeBillingAddressRequest;
}

export namespace StripeBillingAddressRequest {
  export type AsObject = {
    name: string,
    address?: StripeBillingAddress.AsObject,
  }
}

export class StripeBillingAddress extends jspb.Message {
  getAddressCity(): string;
  setAddressCity(value: string): void;

  getAddressCountry(): string;
  setAddressCountry(value: string): void;

  getAddressLine1(): string;
  setAddressLine1(value: string): void;

  getAddressLine2(): string;
  setAddressLine2(value: string): void;

  getAddressPostalCode(): string;
  setAddressPostalCode(value: string): void;

  getAddressState(): string;
  setAddressState(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StripeBillingAddress.AsObject;
  static toObject(includeInstance: boolean, msg: StripeBillingAddress): StripeBillingAddress.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StripeBillingAddress, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StripeBillingAddress;
  static deserializeBinaryFromReader(message: StripeBillingAddress, reader: jspb.BinaryReader): StripeBillingAddress;
}

export namespace StripeBillingAddress {
  export type AsObject = {
    addressCity: string,
    addressCountry: string,
    addressLine1: string,
    addressLine2: string,
    addressPostalCode: string,
    addressState: string,
  }
}

export class UpdateEmailRequest extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateEmailRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateEmailRequest): UpdateEmailRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdateEmailRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateEmailRequest;
  static deserializeBinaryFromReader(message: UpdateEmailRequest, reader: jspb.BinaryReader): UpdateEmailRequest;
}

export namespace UpdateEmailRequest {
  export type AsObject = {
    email: string,
  }
}

export class UpdatePurchaseOrderPrefixRequest extends jspb.Message {
  getPo(): string;
  setPo(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdatePurchaseOrderPrefixRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdatePurchaseOrderPrefixRequest): UpdatePurchaseOrderPrefixRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdatePurchaseOrderPrefixRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdatePurchaseOrderPrefixRequest;
  static deserializeBinaryFromReader(message: UpdatePurchaseOrderPrefixRequest, reader: jspb.BinaryReader): UpdatePurchaseOrderPrefixRequest;
}

export namespace UpdatePurchaseOrderPrefixRequest {
  export type AsObject = {
    po: string,
  }
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
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Card, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Card;
  static deserializeBinaryFromReader(message: Card, reader: jspb.BinaryReader): Card;
}

export namespace Card {
  export type AsObject = {
    id: string,
    last4: string,
    addressLine1: string,
    addressLine2: string,
    city: string,
    country: string,
    state: string,
    name: string,
    zip: string,
    brand: string,
    expirationMonth: number,
    expirationYear: number,
    createdAt: number,
  }
}

export class GetBillingInformationResponse extends jspb.Message {
  getDefaultPaymentMethodId(): string;
  setDefaultPaymentMethodId(value: string): void;

  clearCardsList(): void;
  getCardsList(): Array<Card>;
  setCardsList(value: Array<Card>): void;
  addCards(value?: Card, index?: number): Card;

  getStripePublicKey(): string;
  setStripePublicKey(value: string): void;

  getProductName(): string;
  setProductName(value: string): void;

  getTrial(): boolean;
  setTrial(value: boolean): void;

  getSelfEnrolled(): boolean;
  setSelfEnrolled(value: boolean): void;

  getUpsellAlert(): boolean;
  setUpsellAlert(value: boolean): void;

  getUsageBasedBilling(): boolean;
  setUsageBasedBilling(value: boolean): void;

  getStripeTrial(): boolean;
  setStripeTrial(value: boolean): void;

  getStripeTrialEnd(): number;
  setStripeTrialEnd(value: number): void;

  getStripeMissingPaymentMethod(): boolean;
  setStripeMissingPaymentMethod(value: boolean): void;

  getStripeCustomerId(): string;
  setStripeCustomerId(value: string): void;

  getStripeSubscriptionStatus(): string;
  setStripeSubscriptionStatus(value: string): void;

  getStripeSubscriptionCancelAt(): number;
  setStripeSubscriptionCancelAt(value: number): void;

  getStripeSubscriptionCanceledAt(): number;
  setStripeSubscriptionCanceledAt(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBillingInformationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetBillingInformationResponse): GetBillingInformationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBillingInformationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBillingInformationResponse;
  static deserializeBinaryFromReader(message: GetBillingInformationResponse, reader: jspb.BinaryReader): GetBillingInformationResponse;
}

export namespace GetBillingInformationResponse {
  export type AsObject = {
    defaultPaymentMethodId: string,
    cardsList: Array<Card.AsObject>,
    stripePublicKey: string,
    productName: string,
    trial: boolean,
    selfEnrolled: boolean,
    upsellAlert: boolean,
    usageBasedBilling: boolean,
    stripeTrial: boolean,
    stripeTrialEnd: number,
    stripeMissingPaymentMethod: boolean,
    stripeCustomerId: string,
    stripeSubscriptionStatus: string,
    stripeSubscriptionCancelAt: number,
    stripeSubscriptionCanceledAt: number,
  }
}

export class CreateSetupIntentResponse extends jspb.Message {
  getClientSecret(): string;
  setClientSecret(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateSetupIntentResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CreateSetupIntentResponse): CreateSetupIntentResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateSetupIntentResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateSetupIntentResponse;
  static deserializeBinaryFromReader(message: CreateSetupIntentResponse, reader: jspb.BinaryReader): CreateSetupIntentResponse;
}

export namespace CreateSetupIntentResponse {
  export type AsObject = {
    clientSecret: string,
  }
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

  getUsageMau(): number;
  setUsageMau(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Invoice.AsObject;
  static toObject(includeInstance: boolean, msg: Invoice): Invoice.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Invoice, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Invoice;
  static deserializeBinaryFromReader(message: Invoice, reader: jspb.BinaryReader): Invoice;
}

export namespace Invoice {
  export type AsObject = {
    invoiceId: string,
    status: string,
    amountDue: number,
    amountPaid: number,
    periodEnd: number,
    periodStart: number,
    invoicePdf: string,
    usageMau: number,
  }
}

export class SubmitUsageReportsRequest extends jspb.Message {
  clearReportsList(): void;
  getReportsList(): Array<UsageReport>;
  setReportsList(value: Array<UsageReport>): void;
  addReports(value?: UsageReport, index?: number): UsageReport;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubmitUsageReportsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SubmitUsageReportsRequest): SubmitUsageReportsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubmitUsageReportsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubmitUsageReportsRequest;
  static deserializeBinaryFromReader(message: SubmitUsageReportsRequest, reader: jspb.BinaryReader): SubmitUsageReportsRequest;
}

export namespace SubmitUsageReportsRequest {
  export type AsObject = {
    reportsList: Array<UsageReport.AsObject>,
  }
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
  static toObject(includeInstance: boolean, msg: UsageReport): UsageReport.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UsageReport, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UsageReport;
  static deserializeBinaryFromReader(message: UsageReport, reader: jspb.BinaryReader): UsageReport;
}

export namespace UsageReport {
  export type AsObject = {
    periodstart: number,
    periodend: number,
    itemsList: Array<UsageReportItem.AsObject>,
  }
}

export class UsageReportItem extends jspb.Message {
  getResource(): UsageResourceTypeMap[keyof UsageResourceTypeMap];
  setResource(value: UsageResourceTypeMap[keyof UsageResourceTypeMap]): void;

  getQuantity(): number;
  setQuantity(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UsageReportItem.AsObject;
  static toObject(includeInstance: boolean, msg: UsageReportItem): UsageReportItem.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UsageReportItem, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UsageReportItem;
  static deserializeBinaryFromReader(message: UsageReportItem, reader: jspb.BinaryReader): UsageReportItem;
}

export namespace UsageReportItem {
  export type AsObject = {
    resource: UsageResourceTypeMap[keyof UsageResourceTypeMap],
    quantity: number,
  }
}

export class RemoveCardRequest extends jspb.Message {
  getCardId(): string;
  setCardId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RemoveCardRequest.AsObject;
  static toObject(includeInstance: boolean, msg: RemoveCardRequest): RemoveCardRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RemoveCardRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RemoveCardRequest;
  static deserializeBinaryFromReader(message: RemoveCardRequest, reader: jspb.BinaryReader): RemoveCardRequest;
}

export namespace RemoveCardRequest {
  export type AsObject = {
    cardId: string,
  }
}

export class AddCardRequest extends jspb.Message {
  getCardId(): string;
  setCardId(value: string): void;

  getIsDefault(): boolean;
  setIsDefault(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AddCardRequest.AsObject;
  static toObject(includeInstance: boolean, msg: AddCardRequest): AddCardRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AddCardRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AddCardRequest;
  static deserializeBinaryFromReader(message: AddCardRequest, reader: jspb.BinaryReader): AddCardRequest;
}

export namespace AddCardRequest {
  export type AsObject = {
    cardId: string,
    isDefault: boolean,
  }
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
  static toObject(includeInstance: boolean, msg: UpdateCardRequest): UpdateCardRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdateCardRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateCardRequest;
  static deserializeBinaryFromReader(message: UpdateCardRequest, reader: jspb.BinaryReader): UpdateCardRequest;
}

export namespace UpdateCardRequest {
  export type AsObject = {
    prevCardId: string,
    nextCardId: string,
    isDefault: boolean,
  }
}

export class GetAccountUpgradeWindowStartHourResponse extends jspb.Message {
  getUpgradeWindowStartHour(): number;
  setUpgradeWindowStartHour(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetAccountUpgradeWindowStartHourResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetAccountUpgradeWindowStartHourResponse): GetAccountUpgradeWindowStartHourResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetAccountUpgradeWindowStartHourResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetAccountUpgradeWindowStartHourResponse;
  static deserializeBinaryFromReader(message: GetAccountUpgradeWindowStartHourResponse, reader: jspb.BinaryReader): GetAccountUpgradeWindowStartHourResponse;
}

export namespace GetAccountUpgradeWindowStartHourResponse {
  export type AsObject = {
    upgradeWindowStartHour: number,
  }
}

export class UpdateAccountUpgradeWindowStartHourRequest extends jspb.Message {
  getUpgradeWindowStartHour(): number;
  setUpgradeWindowStartHour(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateAccountUpgradeWindowStartHourRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateAccountUpgradeWindowStartHourRequest): UpdateAccountUpgradeWindowStartHourRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdateAccountUpgradeWindowStartHourRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateAccountUpgradeWindowStartHourRequest;
  static deserializeBinaryFromReader(message: UpdateAccountUpgradeWindowStartHourRequest, reader: jspb.BinaryReader): UpdateAccountUpgradeWindowStartHourRequest;
}

export namespace UpdateAccountUpgradeWindowStartHourRequest {
  export type AsObject = {
    upgradeWindowStartHour: number,
  }
}

export class SendAccountRecoveryLinkRequest extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): void;

  getUrl(): string;
  setUrl(value: string): void;

  getCreatedAt(): number;
  setCreatedAt(value: number): void;

  getIpAddr(): string;
  setIpAddr(value: string): void;

  getUserAgent(): string;
  setUserAgent(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SendAccountRecoveryLinkRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SendAccountRecoveryLinkRequest): SendAccountRecoveryLinkRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SendAccountRecoveryLinkRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SendAccountRecoveryLinkRequest;
  static deserializeBinaryFromReader(message: SendAccountRecoveryLinkRequest, reader: jspb.BinaryReader): SendAccountRecoveryLinkRequest;
}

export namespace SendAccountRecoveryLinkRequest {
  export type AsObject = {
    email: string,
    url: string,
    createdAt: number,
    ipAddr: string,
    userAgent: string,
  }
}

export class SendAccountLockedRequest extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): void;

  getIpAddr(): string;
  setIpAddr(value: string): void;

  getUserAgent(): string;
  setUserAgent(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SendAccountLockedRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SendAccountLockedRequest): SendAccountLockedRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SendAccountLockedRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SendAccountLockedRequest;
  static deserializeBinaryFromReader(message: SendAccountLockedRequest, reader: jspb.BinaryReader): SendAccountLockedRequest;
}

export namespace SendAccountLockedRequest {
  export type AsObject = {
    email: string,
    ipAddr: string,
    userAgent: string,
  }
}

export class SendAccountRecoveredRequest extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): void;

  getRecoveredAt(): number;
  setRecoveredAt(value: number): void;

  getIpAddr(): string;
  setIpAddr(value: string): void;

  getUserAgent(): string;
  setUserAgent(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SendAccountRecoveredRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SendAccountRecoveredRequest): SendAccountRecoveredRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SendAccountRecoveredRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SendAccountRecoveredRequest;
  static deserializeBinaryFromReader(message: SendAccountRecoveredRequest, reader: jspb.BinaryReader): SendAccountRecoveredRequest;
}

export namespace SendAccountRecoveredRequest {
  export type AsObject = {
    email: string,
    recoveredAt: number,
    ipAddr: string,
    userAgent: string,
  }
}

export class GetFeaturesResponse extends jspb.Message {
  getKubernetes(): boolean;
  setKubernetes(value: boolean): void;

  getApp(): boolean;
  setApp(value: boolean): void;

  getDb(): boolean;
  setDb(value: boolean): void;

  getDesktop(): boolean;
  setDesktop(value: boolean): void;

  getModeratedSessions(): boolean;
  setModeratedSessions(value: boolean): void;

  getMachineId(): boolean;
  setMachineId(value: boolean): void;

  getAccessRequests(): boolean;
  setAccessRequests(value: boolean): void;

  getUsageReporting(): boolean;
  setUsageReporting(value: boolean): void;

  getIsCloud(): boolean;
  setIsCloud(value: boolean): void;

  getOidc(): boolean;
  setOidc(value: boolean): void;

  getSaml(): boolean;
  setSaml(value: boolean): void;

  getAccessControls(): boolean;
  setAccessControls(value: boolean): void;

  getHsm(): boolean;
  setHsm(value: boolean): void;

  getIsUsageBased(): boolean;
  setIsUsageBased(value: boolean): void;

  getAssist(): boolean;
  setAssist(value: boolean): void;

  getFeatureHiding(): boolean;
  setFeatureHiding(value: boolean): void;

  getCustomTheme(): string;
  setCustomTheme(value: string): void;

  getIdentityGovernanceSecurity(): boolean;
  setIdentityGovernanceSecurity(value: boolean): void;

  getProductType(): ProductTypeMap[keyof ProductTypeMap];
  setProductType(value: ProductTypeMap[keyof ProductTypeMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetFeaturesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetFeaturesResponse): GetFeaturesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetFeaturesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetFeaturesResponse;
  static deserializeBinaryFromReader(message: GetFeaturesResponse, reader: jspb.BinaryReader): GetFeaturesResponse;
}

export namespace GetFeaturesResponse {
  export type AsObject = {
    kubernetes: boolean,
    app: boolean,
    db: boolean,
    desktop: boolean,
    moderatedSessions: boolean,
    machineId: boolean,
    accessRequests: boolean,
    usageReporting: boolean,
    isCloud: boolean,
    oidc: boolean,
    saml: boolean,
    accessControls: boolean,
    hsm: boolean,
    isUsageBased: boolean,
    assist: boolean,
    featureHiding: boolean,
    customTheme: string,
    identityGovernanceSecurity: boolean,
    productType: ProductTypeMap[keyof ProductTypeMap],
  }
}

export class StripeUsage extends jspb.Message {
  getInvoiceId(): string;
  setInvoiceId(value: string): void;

  getStatus(): string;
  setStatus(value: string): void;

  getPeriodEnd(): number;
  setPeriodEnd(value: number): void;

  getPeriodStart(): number;
  setPeriodStart(value: number): void;

  getUsageMau(): number;
  setUsageMau(value: number): void;

  getUsageTia(): number;
  setUsageTia(value: number): void;

  getUsagePr(): number;
  setUsagePr(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StripeUsage.AsObject;
  static toObject(includeInstance: boolean, msg: StripeUsage): StripeUsage.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StripeUsage, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StripeUsage;
  static deserializeBinaryFromReader(message: StripeUsage, reader: jspb.BinaryReader): StripeUsage;
}

export namespace StripeUsage {
  export type AsObject = {
    invoiceId: string,
    status: string,
    periodEnd: number,
    periodStart: number,
    usageMau: number,
    usageTia: number,
    usagePr: number,
  }
}

export class GetBillingSummaryInformationResponse extends jspb.Message {
  getUsageBasedBilling(): boolean;
  setUsageBasedBilling(value: boolean): void;

  getStripePublicKey(): string;
  setStripePublicKey(value: string): void;

  getStripeCustomerId(): string;
  setStripeCustomerId(value: string): void;

  hasStripeCurrentUsage(): boolean;
  clearStripeCurrentUsage(): void;
  getStripeCurrentUsage(): StripeUsage | undefined;
  setStripeCurrentUsage(value?: StripeUsage): void;

  getStripeTrial(): boolean;
  setStripeTrial(value: boolean): void;

  getStripeTrialEnd(): number;
  setStripeTrialEnd(value: number): void;

  getStripeMissingPaymentMethod(): boolean;
  setStripeMissingPaymentMethod(value: boolean): void;

  getProductName(): string;
  setProductName(value: string): void;

  getStripeSubscriptionStatus(): string;
  setStripeSubscriptionStatus(value: string): void;

  getStripeSubscriptionCancelAt(): number;
  setStripeSubscriptionCancelAt(value: number): void;

  getStripeSubscriptionCanceledAt(): number;
  setStripeSubscriptionCanceledAt(value: number): void;

  getUsageUpdatedAt(): number;
  setUsageUpdatedAt(value: number): void;

  hasUsageQuota(): boolean;
  clearUsageQuota(): void;
  getUsageQuota(): UsageQuota | undefined;
  setUsageQuota(value?: UsageQuota): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBillingSummaryInformationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetBillingSummaryInformationResponse): GetBillingSummaryInformationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBillingSummaryInformationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBillingSummaryInformationResponse;
  static deserializeBinaryFromReader(message: GetBillingSummaryInformationResponse, reader: jspb.BinaryReader): GetBillingSummaryInformationResponse;
}

export namespace GetBillingSummaryInformationResponse {
  export type AsObject = {
    usageBasedBilling: boolean,
    stripePublicKey: string,
    stripeCustomerId: string,
    stripeCurrentUsage?: StripeUsage.AsObject,
    stripeTrial: boolean,
    stripeTrialEnd: number,
    stripeMissingPaymentMethod: boolean,
    productName: string,
    stripeSubscriptionStatus: string,
    stripeSubscriptionCancelAt: number,
    stripeSubscriptionCanceledAt: number,
    usageUpdatedAt: number,
    usageQuota?: UsageQuota.AsObject,
  }
}

export class UsageQuota extends jspb.Message {
  getMauMax(): number;
  setMauMax(value: number): void;

  getTprMax(): number;
  setTprMax(value: number): void;

  getTiaMax(): number;
  setTiaMax(value: number): void;

  getMauInc(): number;
  setMauInc(value: number): void;

  getTprInc(): number;
  setTprInc(value: number): void;

  getTiaInc(): number;
  setTiaInc(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UsageQuota.AsObject;
  static toObject(includeInstance: boolean, msg: UsageQuota): UsageQuota.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UsageQuota, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UsageQuota;
  static deserializeBinaryFromReader(message: UsageQuota, reader: jspb.BinaryReader): UsageQuota;
}

export namespace UsageQuota {
  export type AsObject = {
    mauMax: number,
    tprMax: number,
    tiaMax: number,
    mauInc: number,
    tprInc: number,
    tiaInc: number,
  }
}

export class GetPaymentsInvoicesInformationResponse extends jspb.Message {
  getUsageBasedBilling(): boolean;
  setUsageBasedBilling(value: boolean): void;

  getStripePublicKey(): string;
  setStripePublicKey(value: string): void;

  getStripeCustomerId(): string;
  setStripeCustomerId(value: string): void;

  getStripeMissingPaymentMethod(): boolean;
  setStripeMissingPaymentMethod(value: boolean): void;

  clearStripeCardsList(): void;
  getStripeCardsList(): Array<Card>;
  setStripeCardsList(value: Array<Card>): void;
  addStripeCards(value?: Card, index?: number): Card;

  clearStripeInvoicesList(): void;
  getStripeInvoicesList(): Array<Invoice>;
  setStripeInvoicesList(value: Array<Invoice>): void;
  addStripeInvoices(value?: Invoice, index?: number): Invoice;

  getProductName(): string;
  setProductName(value: string): void;

  getStripeTrialEnd(): number;
  setStripeTrialEnd(value: number): void;

  getStripeDefaultSourceId(): string;
  setStripeDefaultSourceId(value: string): void;

  getStripeSubscriptionStatus(): string;
  setStripeSubscriptionStatus(value: string): void;

  getStripeSubscriptionCancelAt(): number;
  setStripeSubscriptionCancelAt(value: number): void;

  getStripeSubscriptionCanceledAt(): number;
  setStripeSubscriptionCanceledAt(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetPaymentsInvoicesInformationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetPaymentsInvoicesInformationResponse): GetPaymentsInvoicesInformationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetPaymentsInvoicesInformationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetPaymentsInvoicesInformationResponse;
  static deserializeBinaryFromReader(message: GetPaymentsInvoicesInformationResponse, reader: jspb.BinaryReader): GetPaymentsInvoicesInformationResponse;
}

export namespace GetPaymentsInvoicesInformationResponse {
  export type AsObject = {
    usageBasedBilling: boolean,
    stripePublicKey: string,
    stripeCustomerId: string,
    stripeMissingPaymentMethod: boolean,
    stripeCardsList: Array<Card.AsObject>,
    stripeInvoicesList: Array<Invoice.AsObject>,
    productName: string,
    stripeTrialEnd: number,
    stripeDefaultSourceId: string,
    stripeSubscriptionStatus: string,
    stripeSubscriptionCancelAt: number,
    stripeSubscriptionCanceledAt: number,
  }
}

export class GetInvoiceSettingsInformationResponse extends jspb.Message {
  getUsageBasedBilling(): boolean;
  setUsageBasedBilling(value: boolean): void;

  getStripePublicKey(): string;
  setStripePublicKey(value: string): void;

  getStripeCustomerId(): string;
  setStripeCustomerId(value: string): void;

  hasStripeInvoiceBillingAddress(): boolean;
  clearStripeInvoiceBillingAddress(): void;
  getStripeInvoiceBillingAddress(): StripeBillingAddress | undefined;
  setStripeInvoiceBillingAddress(value?: StripeBillingAddress): void;

  getStripeInvoiceEmail(): string;
  setStripeInvoiceEmail(value: string): void;

  getStripeInvoicePurchaseOrderNumber(): string;
  setStripeInvoicePurchaseOrderNumber(value: string): void;

  getStripeCustomerName(): string;
  setStripeCustomerName(value: string): void;

  getStripeSubscriptionStatus(): string;
  setStripeSubscriptionStatus(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetInvoiceSettingsInformationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetInvoiceSettingsInformationResponse): GetInvoiceSettingsInformationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetInvoiceSettingsInformationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetInvoiceSettingsInformationResponse;
  static deserializeBinaryFromReader(message: GetInvoiceSettingsInformationResponse, reader: jspb.BinaryReader): GetInvoiceSettingsInformationResponse;
}

export namespace GetInvoiceSettingsInformationResponse {
  export type AsObject = {
    usageBasedBilling: boolean,
    stripePublicKey: string,
    stripeCustomerId: string,
    stripeInvoiceBillingAddress?: StripeBillingAddress.AsObject,
    stripeInvoiceEmail: string,
    stripeInvoicePurchaseOrderNumber: string,
    stripeCustomerName: string,
    stripeSubscriptionStatus: string,
  }
}

export class MarketingParamData extends jspb.Message {
  getCampaign(): string;
  setCampaign(value: string): void;

  getSource(): string;
  setSource(value: string): void;

  getMedium(): string;
  setMedium(value: string): void;

  getIntent(): string;
  setIntent(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MarketingParamData.AsObject;
  static toObject(includeInstance: boolean, msg: MarketingParamData): MarketingParamData.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MarketingParamData, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MarketingParamData;
  static deserializeBinaryFromReader(message: MarketingParamData, reader: jspb.BinaryReader): MarketingParamData;
}

export namespace MarketingParamData {
  export type AsObject = {
    campaign: string,
    source: string,
    medium: string,
    intent: string,
  }
}

export class SurveyCompanyResponse extends jspb.Message {
  getCompanyName(): string;
  setCompanyName(value: string): void;

  getEmployeeCount(): string;
  setEmployeeCount(value: string): void;

  hasMarketingParams(): boolean;
  clearMarketingParams(): void;
  getMarketingParams(): MarketingParamData | undefined;
  setMarketingParams(value?: MarketingParamData): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SurveyCompanyResponse.AsObject;
  static toObject(includeInstance: boolean, msg: SurveyCompanyResponse): SurveyCompanyResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SurveyCompanyResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SurveyCompanyResponse;
  static deserializeBinaryFromReader(message: SurveyCompanyResponse, reader: jspb.BinaryReader): SurveyCompanyResponse;
}

export namespace SurveyCompanyResponse {
  export type AsObject = {
    companyName: string,
    employeeCount: string,
    marketingParams?: MarketingParamData.AsObject,
  }
}

export class SetSurveyResultsRequest extends jspb.Message {
  getCompanyName(): string;
  setCompanyName(value: string): void;

  getEmployeeCount(): string;
  setEmployeeCount(value: string): void;

  clearResourcesList(): void;
  getResourcesList(): Array<string>;
  setResourcesList(value: Array<string>): void;
  addResources(value: string, index?: number): string;

  getRole(): string;
  setRole(value: string): void;

  getTeam(): string;
  setTeam(value: string): void;

  getUsername(): string;
  setUsername(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SetSurveyResultsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SetSurveyResultsRequest): SetSurveyResultsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SetSurveyResultsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SetSurveyResultsRequest;
  static deserializeBinaryFromReader(message: SetSurveyResultsRequest, reader: jspb.BinaryReader): SetSurveyResultsRequest;
}

export namespace SetSurveyResultsRequest {
  export type AsObject = {
    companyName: string,
    employeeCount: string,
    resourcesList: Array<string>,
    role: string,
    team: string,
    username: string,
  }
}

export class SendTeleportInviteRequest extends jspb.Message {
  getRecipient(): string;
  setRecipient(value: string): void;

  getInviteUrl(): string;
  setInviteUrl(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SendTeleportInviteRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SendTeleportInviteRequest): SendTeleportInviteRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SendTeleportInviteRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SendTeleportInviteRequest;
  static deserializeBinaryFromReader(message: SendTeleportInviteRequest, reader: jspb.BinaryReader): SendTeleportInviteRequest;
}

export namespace SendTeleportInviteRequest {
  export type AsObject = {
    recipient: string,
    inviteUrl: string,
  }
}

export class EmptyResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EmptyResponse.AsObject;
  static toObject(includeInstance: boolean, msg: EmptyResponse): EmptyResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EmptyResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EmptyResponse;
  static deserializeBinaryFromReader(message: EmptyResponse, reader: jspb.BinaryReader): EmptyResponse;
}

export namespace EmptyResponse {
  export type AsObject = {
  }
}

export class EmptyRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EmptyRequest.AsObject;
  static toObject(includeInstance: boolean, msg: EmptyRequest): EmptyRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EmptyRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EmptyRequest;
  static deserializeBinaryFromReader(message: EmptyRequest, reader: jspb.BinaryReader): EmptyRequest;
}

export namespace EmptyRequest {
  export type AsObject = {
  }
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
  OTHER: 8;
}

export const UsageResourceType: UsageResourceTypeMap;

export interface ProductTypeMap {
  PRODUCT_TYPE_UNKNOWN: 0;
  PRODUCT_TYPE_TEAM: 1;
  PRODUCT_TYPE_EUB: 2;
}

export const ProductType: ProductTypeMap;

