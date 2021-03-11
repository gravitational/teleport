import React, { useState } from 'react';
import { Flex, ButtonPrimary, ButtonSecondary } from 'design';
import { Danger } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { requiredField } from 'shared/components/Validation/rules';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';
import {
  Account,
  countryMap,
  countryOptions,
  CountryCode,
  CountryOption,
} from 'e-teleport/services/cloud';

export default function AccountEditor({
  attempt,
  account,
  onClose,
  onSave,
}: Props) {
  const [companyName, setCompanyName] = useState(account.companyName);
  const [contactEmail, setEmail] = useState(account.contactEmail);
  const [companyAddressLine1, setAddress] = useState(
    account.companyAddressLine1
  );
  const [companyAddressCity, setCity] = useState(account.companyAddressCity);
  const [companyAddressState, setState] = useState(account.companyAddressState);
  const [companyAddressPostalCode, setZipCode] = useState(
    account.companyAddressPostalCode
  );
  const [companyAddressCountry, setCountry] = useState(
    account.companyAddressCountry as CountryCode
  );

  function handleOnSave(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    onSave({
      ...account,
      companyName,
      contactEmail,
      companyAddressLine1,
      companyAddressCity,
      companyAddressState,
      companyAddressPostalCode,
      companyAddressCountry,
    } as Account);
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          disableEscapeKeyDown={false}
          open={true}
          onClose={onClose}
          dialogCss={() => ({
            maxWidth: '600px',
            width: '100%',
          })}
        >
          <DialogHeader>
            <DialogTitle>Billing Information</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Danger>{attempt.statusText}</Danger>
            )}
            <FieldInput
              autoFocus={true}
              type="text"
              label="Company Name"
              value={companyName}
              onChange={e => setCompanyName(e.target.value)}
              placeholder="Optional"
            />
            <FieldInput
              rule={requiredField('Billing Email is required')}
              type="email"
              label="Billing Email"
              value={contactEmail}
              onChange={e => setEmail(e.target.value)}
              placeholder="Billing Email"
            />
            <FieldInput
              type="text"
              label="Billing Address"
              value={companyAddressLine1}
              setAddress
              onChange={e => setAddress(e.target.value)}
              placeholder="123 Acme Way"
            />
            <Flex>
              <FieldInput
                type="text"
                label="City"
                value={companyAddressCity}
                onChange={e => setCity(e.target.value)}
                placeholder="City"
                mr={3}
                width="40%"
              />
              <FieldInput
                type="text"
                label="State"
                value={companyAddressState}
                onChange={e => setState(e.target.value)}
                placeholder="State"
                mr={3}
                width="30%"
              />
              <FieldInput
                type="text"
                label="Zip Code"
                value={companyAddressPostalCode}
                onChange={e => setZipCode(e.target.value)}
                placeholder="Zip Code"
                width="30%"
              />
            </Flex>
            <FieldSelect
              clearable={true}
              menuPosition="fixed"
              isSimpleValue={true}
              label="Country"
              value={
                companyAddressCountry
                  ? {
                      value: companyAddressCountry,
                      label: countryMap[companyAddressCountry],
                    }
                  : null
              }
              onChange={(e: CountryOption) => setCountry(e.value)}
              placeholder="Select Country"
              options={countryOptions}
              isSearchable
              maxMenuHeight={200}
            />
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              disabled={attempt.status === 'processing'}
              onClick={() => handleOnSave(validator)}
              mr="3"
            >
              save
            </ButtonPrimary>
            <ButtonSecondary
              disabled={attempt.status === 'processing'}
              onClick={onClose}
            >
              cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}
interface Props {
  attempt: Attempt;
  account: Account;
  onSave(account: Account): void;
  onClose(): void;
}
