import React from 'react';
import CardError, {
  NotFound,
  AccessDenied,
  Failed,
  Offline,
  LoginFailed,
} from './index';
import { render } from 'design/utils/testing';

const msg = 'some message';

describe('design/CardError', () => {
  it('respects styles', () => {
    const { container } = render(<CardError />);
    expect(container.firstChild).toHaveStyle('width: 540px');
  });

  test('<NotFound> renders header text and respects message prop', () => {
    const { container } = render(<NotFound message={msg} />);

    expect(container.firstChild.children[0].textContent).toBe('404 Not Found');
    expect(container.firstChild.children[1].textContent).toBe(msg);
  });

  test('<AccessDenied> renders header text and respects message prop', () => {
    const { container } = render(<AccessDenied message={msg} />);

    expect(container.firstChild.children[0].textContent).toBe('Access Denied');
    expect(container.firstChild.children[1].textContent).toBe(msg);
  });

  test('<Failed> renders header text and respects message prop', () => {
    const { container } = render(<Failed message={msg} />);

    expect(container.firstChild.children[0].textContent).toBe('Internal Error');
    expect(container.firstChild.children[1].textContent).toBe(msg);
  });

  test('<Offline> respects title and message props', () => {
    const title = 'some title';
    const { container } = render(<Offline title={title} message={msg} />);
    expect(container.firstChild.children[0].textContent).toBe(title);
    expect(container.firstChild.children[1].textContent).toBe(msg);
  });

  test('<LoginFailed> renders header text, respects message & href prop', () => {
    const url = 'someURL';
    const { container } = render(<LoginFailed message={msg} loginUrl={url} />);

    expect(container.firstChild.children[0].textContent).toBe(
      'Login Unsuccessful'
    );
    expect(container.firstChild.children[1].textContent).toBe(msg);
    expect(container.firstChild.children[2].textContent).toBe(
      'Please try again, if the problem persists, contact your system administrator.'
    );
    expect(container.querySelector('a').getAttribute('href')).toBe(url);
  });
});
