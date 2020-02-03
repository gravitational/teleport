import React from 'react';
import CardSuccess from './index';
import { render } from 'design/utils/testing';

describe('Design/CardSuccess', () => {
  it('renders checkmark icon', () => {
    const { container } = render(<CardSuccess />);

    expect(
      container
        .querySelector('span')
        .classList.contains('icon-checkmark-circle')
    ).toBeTruthy();
  });

  it('respects title prop and render text children', () => {
    const title = 'some title';
    const text = 'some text';
    const { container } = render(
      <CardSuccess title={title}>{text}</CardSuccess>
    );

    expect(container.firstChild.children[1].textContent).toBe(title);
    expect(container.firstChild.children[2].textContent).toBe(text);
  });
});
