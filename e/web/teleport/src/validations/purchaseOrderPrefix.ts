/**
 * Returns true if the string is a valid po prefix
 *
 * @remarks
 * Must be 3–12 letters or numbers
 * https://stripe.com/docs/api/customers/create#create_customer-invoice_prefix
 *
 * @param po - string value to be validated
 * @returns boolean value of validation
 */
export const isValidPurchaseOrderPrefix = (po: string): boolean => {
  const regex = new RegExp(`^[a-zA-Z0-9]{3,12}$`);
  return regex.test(po);
};
