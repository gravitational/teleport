// tell eslint to ignore auto-generated proto files
/*eslint import/no-unresolved : 0 */
import cloud from './v1/tenants_pb';

export type Account = cloud.Account.AsObject;

export type BillingInformation = cloud.GetBillingInformationResponse.AsObject;

export type Status = 'PENDING' | 'PAID';

export type Invoice = cloud.Invoice.AsObject;

export type CreditCard = cloud.Card.AsObject;

export type AddCardRequest = cloud.AddCardRequest.AsObject;

export type RemoveCardRequest = cloud.RemoveCardRequest.AsObject;

export type UpdateCardRequest = cloud.UpdateCardRequest.AsObject;

export type UpdateAccountRequest = cloud.UpdateAccountRequest.AsObject;

export type BillingCycle = cloud.BillingCycle.AsObject;

export type BillingCycleStatus = 'started' | 'ended';
