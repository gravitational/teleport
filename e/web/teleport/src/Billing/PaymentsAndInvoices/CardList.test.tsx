import React from 'react';

import { screen, userEvent } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { CardList } from 'e-teleport/Billing/PaymentsAndInvoices/CardList';
import { CardsListProps } from 'e-teleport/Billing/types';

describe('cardList', () => {
  let props: CardsListProps;
  const defaultCard = {
    id: 'some-id',
    last4: '9999',
    addressLine1: 'some-addressLine1',
    addressLine2: 'some-addressLine2',
    city: 'some-city',
    country: 'some-country',
    state: 'some-state',
    name: 'some-name',
    zip: 'some-zip',
    brand: 'some-brand',
    expirationMonth: 12,
    expirationYear: 2023,
    createdAt: 1682989632,
  };

  beforeEach(() => {
    props = {
      cards: [],
      setPageState: jest.fn(),
    };
  });

  test('renders the card list', () => {
    props.cards = [defaultCard];
    renderWithElementsAndContext(<CardList {...props} />);

    expect(screen.getByText(/Payment Methods/i)).toBeInTheDocument();
    expect(
      screen.getByText(/At most, three credit cards can be added./i)
    ).toBeInTheDocument();

    expect(
      screen.getByText(/\*\*\*\* \*\*\*\* \*\*\*\* 9999/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/some-name/i)).toBeInTheDocument();
    expect(screen.getByText(/12\/2023/i)).toBeInTheDocument();
  });

  describe('add payment', () => {
    // eslint-disable-next-line jest/require-hook
    [
      {
        length: 0,
        cardsList: [],
      },
      {
        length: 1,
        cardsList: [defaultCard],
      },
      {
        length: 2,
        cardsList: [defaultCard, { ...defaultCard, id: 'some-other-id' }],
      },
    ].forEach(spec => {
      test(`renders when card list length is ${spec.length}`, () => {
        props.cards = spec.cardsList;
        renderWithElementsAndContext(<CardList {...props} />);

        expect(screen.getByText(/Add a Payment Method/i)).toBeInTheDocument();
      });
    });

    test('does not render when cards list length is 3', () => {
      props.cards = [
        defaultCard,
        { ...defaultCard, id: 'some-other-id' },
        { ...defaultCard, id: 'some-other-other-id' },
      ];
      renderWithElementsAndContext(<CardList {...props} />);

      expect(
        screen.queryByText(/Add a Payment Method/i)
      ).not.toBeInTheDocument();
    });

    test('clicking button opens modal', async () => {
      renderWithElementsAndContext(<CardList {...props} />);

      expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();
      await userEvent.click(
        screen.getByRole('button', { name: 'Add a Payment Method' })
      );
      expect(screen.getByTestId('Modal')).toBeInTheDocument();
    });
  });

  describe('icons', () => {
    // eslint-disable-next-line jest/require-hook
    [
      { brand: 'visa', expected: 'icon-visa' },
      { brand: 'amex', expected: 'icon-amex' },
      { brand: 'mastercard', expected: 'icon-mastercard' },
      { brand: 'discover', expected: 'icon-discover' },
      { brand: 'diners', expected: 'icon-generic' },
      { brand: 'foo', expected: 'icon-unknown' },
    ].forEach(spec => {
      test(`should display the ${spec.brand} icon`, () => {
        const specCard = { ...defaultCard, brand: spec.brand };
        props.cards = [specCard];
        renderWithElementsAndContext(<CardList {...props} />);

        expect(screen.getByTestId(spec.expected)).toBeInTheDocument();
      });
    });
  });

  describe('delete action', () => {
    test('is disabled for one card', () => {
      props.cards = [defaultCard];
      renderWithElementsAndContext(<CardList {...props} />);

      expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled();
    });

    test('clicking delete opens modal', async () => {
      props.cards = [defaultCard, { ...defaultCard, id: 'some-other-id' }];
      renderWithElementsAndContext(<CardList {...props} />);

      expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();
      await userEvent.click(
        screen.getAllByRole('button', { name: 'Delete' })[1]
      );
      expect(screen.getByTestId('Modal')).toBeInTheDocument();
    });
  });

  describe('make default action', () => {
    test('is disabled for one card', () => {
      props.cards = [defaultCard];
      props.defaultSourceID = defaultCard.id;
      renderWithElementsAndContext(<CardList {...props} />);

      expect(
        screen.getByRole('button', { name: /Make Default/i })
      ).toBeDisabled();
    });

    test('enabled if cardList length > 1', () => {
      props.cards = [defaultCard, { ...defaultCard, id: 'some-other-id' }];
      props.defaultSourceID = defaultCard.id;
      renderWithElementsAndContext(<CardList {...props} />);

      expect(
        screen.getAllByRole('button', { name: /Make Default/i })[0]
      ).toBeDisabled();
      expect(
        screen.getAllByRole('button', { name: /Make Default/i })[1]
      ).toBeEnabled();
    });
  });
});
