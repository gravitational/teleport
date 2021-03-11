/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import { Text, Flex } from 'design';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import * as Alerts from 'design/Alert';
import { StateSuccess } from 'design/LabelState';
import { CreditCard } from 'e-teleport/services/cloud';
import LightThemeProvider from './ThemeProvider';

export default function CrediCardList(props: Props) {
  const { cards = [], maxWidth, mb, onRemove, onSetDefault, attempt } = props;
  const $items = cards.map(card => (
    <CreditCardItem
      isDefault={card.id === props.defaultPaymentMethodId}
      mb={4}
      ml={5}
      card={card}
      key={card.id}
      onSetDefault={onSetDefault}
      onRemove={onRemove}
    />
  ));

  // a fancy and styled create new card button
  $items.push(<NewCardButtonItem ml={5} key="add-btn" onClick={props.onNew} />);

  return (
    <Flex maxWidth={maxWidth} mb={mb} flexDirection="column">
      {attempt.status === 'failed' && (
        <Alerts.Danger width="100%" children={attempt.statusText} />
      )}
      <Flex flex={1} ml={-5} flexWrap="wrap">
        {$items}
      </Flex>
    </Flex>
  );
}

function CreditCardItem(props: ItemProps) {
  const { card, onRemove, isDefault, onSetDefault, ...rest } = props;
  return (
    <LightThemeProvider>
      <StyledCreditCardItem
        p={4}
        flexDirection="column"
        bg="primary.light"
        color="text.primary"
        {...rest}
      >
        <Flex width="100%" justifyContent="center">
          <MenuIcon buttonIconProps={menuActionProps}>
            <MenuItem onClick={() => onSetDefault(card.id)}>
              Make Primary
            </MenuItem>
            <MenuItem onClick={() => onRemove(card.id)}>Delete</MenuItem>
          </MenuIcon>
        </Flex>

        <Flex mb={4}>
          <Text typography="h6" bold mr={3}>
            CREDIT CARD
          </Text>
          {isDefault && <StateSuccess width="80px">Primary</StateSuccess>}
        </Flex>
        <Text bold color="text.secondary" fontSize={5} mb={7}>
          XXXX&nbsp;&nbsp;&nbsp;&nbsp;XXXX&nbsp;&nbsp;&nbsp;&nbsp;XXXX&nbsp;&nbsp;&nbsp;&nbsp;
          {card.last4}
        </Text>
        <Flex justifyContent="space-between">
          <Text
            typography="h5"
            color="text.secondary"
            bold
            mr={4}
            style={{
              textTransform: 'uppercase',
              maxWidth: '240px',
            }}
          >
            {card.brand}
          </Text>
          <Text typography="h5" color="text.secondary">
            {`${card.expirationMonth}/${card.expirationYear}`}
          </Text>
        </Flex>
      </StyledCreditCardItem>
    </LightThemeProvider>
  );
}

function NewCardButtonItem(props: any) {
  return (
    <StyledCreditCardItem
      p={4}
      as="button"
      flexDirection="column"
      bg="primary.light"
      color="text.secondary"
      style={{
        border: '2px dotted',
        cursor: 'pointer',
      }}
      {...props}
    >
      <Text typography="h6" bold>
        CREDIT CARD
      </Text>
      <Flex
        as={Text}
        mt={-3}
        mx="auto"
        alignItems="center"
        flex="1"
        typography="h6"
        bold
        color="text.primary"
      >
        ADD A CREDIT CARD
      </Flex>
    </StyledCreditCardItem>
  );
}

const StyledCreditCardItem = styled(Flex)`
  position: relative;
  border: 2px solid transparent;
  border-radius: 16px;
  width: 200px;
  height: 200px;
  position: relative;
  width: 340px;
  background: ${props => props.theme.colors.white};
`;

type Props = {
  defaultPaymentMethodId: string;
  onSetDefault(cardId: string): void;
  onNew(): void;
  onRemove(cardId: string): void;
  cards: CreditCard[];
  maxWidth: string;
  mb: number;
  attempt: Attempt;
};

type ItemProps = {
  onSetDefault(cardId: string): void;
  onRemove(cardId: string): void;
  isDefault: boolean;
  card: CreditCard;
  mb?: number;
  ml?: number;
};

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};
