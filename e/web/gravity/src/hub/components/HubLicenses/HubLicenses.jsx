import React from 'react';
import styled from 'styled-components';
import { isDate } from 'lodash';
import { withState, useAttempt } from 'shared/hooks';
import * as Icons from 'design/Icon';
import { Danger } from 'design/Alert';
import {
  Box,
  Flex,
  Card,
  Input,
  Text,
  LabelInput,
  ButtonPrimary,
} from 'design';
import Validation, {
  useValidation,
  useRule,
} from 'shared/components/Validation';
import htmlUtils from 'gravity/lib/htmlUtils';
import Checkbox from 'e-gravity/hub/components/components/Checkbox';
import { createLicense } from 'e-gravity/hub/services/license';
import ExpirationDate from './ExpirationDate';
import LicenseTextAre from './LicenseTextArea';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from '../components/Layout';

export function HubLicenses(props) {
  const {
    attempt,
    license,
    onSetLicense,
    attemptActions,
    onCreateLicense,
  } = props;
  const [isStrict, setStrict] = React.useState(true);
  const [amount, onChangeAmount] = React.useState('');
  const [expiration, onChangeExpiration] = React.useState(null);
  const licenseRef = React.useRef();

  function changeStrict(value) {
    setStrict(value);
  }

  const onSubmit = () => {
    onSetLicense(null);
    attemptActions.do(() => {
      return onCreateLicense({
        amount,
        expiration,
        isStrict,
      }).then(newLicense => {
        onSetLicense(newLicense);
      });
    });
  };

  function onCopy() {
    event.preventDefault();
    htmlUtils.copyToClipboard(license);
    htmlUtils.selectElementContent(licenseRef.current);
  }

  const { isProcessing, isFailed, message } = attempt;

  return (
    <Validation>
      <FeatureBox>
        <FeatureHeader>
          <FeatureHeaderTitle>Licenses</FeatureHeaderTitle>
        </FeatureHeader>
        <Flex flexWrap="wrap">
          <Card
            flexBasis="400px"
            p="4"
            mr="4"
            mb="4"
            minWidth="200px"
            alignSelf="flex-start"
          >
            {isFailed && <Danger mb="4">{message}</Danger>}
            <Flex>
              <Box mr="3" flex="1" mb="4">
                <Label
                  rule={validNumberOfNodes}
                  value={amount}
                  title={'Max Number of Nodes'}
                />
                <Input
                  autoComplete="off"
                  min="0"
                  value={amount}
                  type="number"
                  onChange={e => onChangeAmount(new Number(e.target.value))}
                />
              </Box>
              <Box flex="1" mb="4">
                <Label
                  rule={validDate}
                  value={expiration}
                  title={'Expiration Date'}
                />
                <ExpirationDate
                  value={expiration}
                  onChange={onChangeExpiration}
                  type="text"
                />
              </Box>
            </Flex>
            <Checkbox
              color="text.primary"
              mb="3"
              label="Stop the application when license expires"
              value={isStrict}
              onChange={changeStrict}
            />
            <CreateButton block disabled={isProcessing} onClick={onSubmit}>
              Generate License
            </CreateButton>
          </Card>
          {license && (
            <Card
              bg="white"
              color="black"
              p="4"
              as={Flex}
              flex="1"
              minWidth="800px"
            >
              <Flex
                width="400px"
                flexDirection="column"
                mr="4"
                justifyContent="space-between"
              >
                <Box>
                  <Flex alignItems="center" mb="3">
                    <Icons.License color="inherit" fontSize={8} mr={2} />
                    <Text typography="h4" color="text.onLight" as="span">
                      CLUSTER LICENSE
                    </Text>
                  </Flex>
                  <Text typography="h5" mt={3}>
                    INSTRUCTIONS (command line)
                  </Text>
                  <Text typography="body1" mt={3}>
                    1. Copy the license and save it as a{' '}
                    <StyledCode>license.pem</StyledCode> file
                  </Text>
                  <Text typography="body1" mt={2}>
                    2. Run the following command <br />
                    <StyledCode>
                      gravity install --license=$(cat license.pem)
                    </StyledCode>
                  </Text>
                </Box>
                <ButtonPrimary onClick={onCopy}>Click to Copy</ButtonPrimary>
              </Flex>
              <LicenseTextAre
                width="100%"
                ref={licenseRef}
                text={license}
                style={{ maxHeight: '300px', overflow: 'auto' }}
              />
            </Card>
          )}
        </Flex>
      </FeatureBox>
    </Validation>
  );
}

const StyledCode = styled.code`
  color: ${props => props.theme.colors.info};
  font-family: ${props => props.theme.fonts.mono};
  font-size: 12px;
`;

function Label({ title, value, rule }) {
  const { valid, message } = useRule(rule(value));
  const hasError = !valid;
  const labelText = hasError ? message : title;
  return <LabelInput hasError={hasError}>{labelText}</LabelInput>;
}

function CreateButton({ onClick, disabled }) {
  const validator = useValidation();
  function onCreate() {
    validator.validate() && onClick();
  }

  return (
    <ButtonPrimary block disabled={disabled} onClick={onCreate}>
      Generate License
    </ButtonPrimary>
  );
}

const validNumberOfNodes = value => () => {
  let message = '';
  if (!value) {
    message = 'required field';
  } else if (value <= 0) {
    message = ' should be greater than 0';
  }

  return {
    valid: message.length === 0,
    message,
  };
};

const validDate = value => () => {
  if (!value || !isDate(value)) {
    return {
      valid: false,
      message: 'invalid date',
    };
  }

  return {
    valid: true,
  };
};

function mapState() {
  const [attempt, attemptActions] = useAttempt();
  const [license, onSetLicense] = React.useState(null);

  return {
    onCreateLicense: createLicense,
    onSetLicense,
    attempt,
    attemptActions,
    license,
  };
}

export default withState(mapState)(HubLicenses);
