import React, {
  forwardRef,
  ReactNode,
  useCallback,
  useImperativeHandle,
} from 'react';
import { act, fireEvent, render, screen } from 'design/utils/testing';

import { KeyboardArrowsNavigation } from './KeyboardArrowsNavigation';
import {
  useKeyboardArrowsNavigation,
  useKeyboardArrowsNavigationStateUpdate,
} from './useKeyboardArrowsNavigation';

function createTextItem(index: number, isActive: boolean) {
  return `Index: ${index} active ${isActive.toString()}`;
}

function getAllItemsText(activeIndex: number, length: number) {
  return Array.from(new Array(length))
    .fill(0)
    .map((_, index) => createTextItem(index, index === activeIndex))
    .join('');
}

function TestItem(props: { index: number }) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: useCallback(() => {}, []),
  });

  return <>{createTextItem(props.index, isActive)}</>;
}

test('context should render provided children', () => {
  render(
    <KeyboardArrowsNavigation>
      <span>Children</span>
    </KeyboardArrowsNavigation>
  );

  expect(screen.getByText('Children')).toBeVisible();
});

test('none of items is active by default', () => {
  const { container } = render(
    <KeyboardArrowsNavigation>
      <TestItem index={0} />
    </KeyboardArrowsNavigation>
  );

  expect(container).toHaveTextContent(getAllItemsText(-1, 1));
});

describe('go through navigation items', () => {
  test('in down direction', () => {
    const { container } = render(
      <KeyboardArrowsNavigation>
        <TestItem index={0} />
        <TestItem index={1} />
        <TestItem index={2} />
      </KeyboardArrowsNavigation>
    );

    fireEvent.keyDown(container.firstChild, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(0, 3));

    fireEvent.keyDown(container.firstChild, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(1, 3));

    fireEvent.keyDown(container.firstChild, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(2, 3));

    fireEvent.keyDown(container.firstChild, { key: 'ArrowDown' });
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

    fireEvent.keyDown(container.firstChild, { key: 'ArrowDown' });
    expect(container).toHaveTextContent(getAllItemsText(0, 3));

    fireEvent.keyDown(container.firstChild, { key: 'ArrowUp' });
    expect(container).toHaveTextContent(getAllItemsText(2, 3));

    fireEvent.keyDown(container.firstChild, { key: 'ArrowUp' });
    expect(container).toHaveTextContent(getAllItemsText(1, 3));

    fireEvent.keyDown(container.firstChild, { key: 'ArrowUp' });
    expect(container).toHaveTextContent(getAllItemsText(0, 3));
  });
});

test('fire action on active item when Enter is pressed', () => {
  const firstItemCallback = jest.fn();

  function TestItem(props: { index: number; onRunActiveItem(): void }) {
    useKeyboardArrowsNavigation({
      index: props.index,
      onRun: props.onRunActiveItem,
    });

    return <>Test item</>;
  }

  const { container } = render(
    <KeyboardArrowsNavigation>
      <TestItem index={0} onRunActiveItem={firstItemCallback} />
    </KeyboardArrowsNavigation>
  );

  fireEvent.keyDown(container.firstChild, { key: 'ArrowDown' });
  fireEvent.keyDown(container.firstChild, { key: 'Enter' });
  expect(firstItemCallback).toHaveBeenCalledWith();
});

test('activeIndex can be changed manually', () => {
  const Container = forwardRef<any, { children: ReactNode }>(
    (props, forwardedRef) => {
      const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();

      useImperativeHandle(forwardedRef, () => ({
        setActiveIndex,
      }));

      return <>{props.children}</>;
    }
  );

  const ref = React.createRef<any>();

  const { container } = render(
    <KeyboardArrowsNavigation>
      <Container ref={ref}>
        <TestItem index={0} />
        <TestItem index={1} />
      </Container>
    </KeyboardArrowsNavigation>
  );

  act(() => ref.current.setActiveIndex(1));
  expect(container).toHaveTextContent(getAllItemsText(1, 2));
});
