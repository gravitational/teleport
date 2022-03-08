import React, { useCallback } from 'react';
import { fireEvent, render } from 'design/utils/testing';
import { KeyboardArrowsNavigation } from './KeyboardArrowsNavigation';
import { useKeyboardArrowsNavigation } from './useKeyboardArrowsNavigation';

test('context should render provided children', () => {
  const { getByText } = render(
    <KeyboardArrowsNavigation>
      <span>Children</span>
    </KeyboardArrowsNavigation>
  );

  expect(getByText('Children')).toBeVisible();
});

describe('should go through navigation items', () => {
  function createTextItem(index: number, isActive: boolean) {
    return `Index: ${index} active ${isActive.toString()}`;
  }

  function TestItem(props: { index: number }) {
    const { isActive } = useKeyboardArrowsNavigation({
      index: props.index,
      onRunActiveItem: useCallback(() => {}, []),
    });

    return <>{createTextItem(props.index, isActive)}</>;
  }

  function getAllItemsText(activeIndex: number, length: number) {
    return Array.from(new Array(length))
      .fill(0)
      .map((_, index) => createTextItem(index, index === activeIndex))
      .join('');
  }

  test('in down direction', () => {
    const { container } = render(
      <KeyboardArrowsNavigation>
        <TestItem index={0} />
        <TestItem index={1} />
        <TestItem index={2} />
      </KeyboardArrowsNavigation>
    );

    expect(container).toHaveTextContent(getAllItemsText(0, 3));

    fireEvent.keyDown(window, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(1, 3));

    fireEvent.keyDown(window, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(2, 3));

    fireEvent.keyDown(window, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(0, 3));
  });

  test('in up direction', () => {
    const { container } = render(
      <KeyboardArrowsNavigation>
        <TestItem index={0} />
        <TestItem index={1} />
        <TestItem index={2} />
      </KeyboardArrowsNavigation>
    );

    expect(container).toHaveTextContent(getAllItemsText(0, 3));

    fireEvent.keyDown(window, { key: 'ArrowUp' });
    expect(container).toHaveTextContent(getAllItemsText(2, 3));

    fireEvent.keyDown(window, { key: 'ArrowUp' });
    expect(container).toHaveTextContent(getAllItemsText(1, 3));

    fireEvent.keyDown(window, { key: 'ArrowUp' });
    expect(container).toHaveTextContent(getAllItemsText(0, 3));
  });
});

test('should fire action on active item when Enter is pressed', () => {
  const firstItemCallback = jest.fn();

  function TestItem(props: { index: number; onRunActiveItem(): void }) {
    useKeyboardArrowsNavigation({
      index: props.index,
      onRunActiveItem: props.onRunActiveItem,
    });

    return <>Test item</>;
  }

  render(
    <KeyboardArrowsNavigation>
      <TestItem index={0} onRunActiveItem={firstItemCallback} />
    </KeyboardArrowsNavigation>
  );
  fireEvent.keyDown(window, { key: 'Enter' });
  expect(firstItemCallback).toHaveBeenCalledWith();
});
