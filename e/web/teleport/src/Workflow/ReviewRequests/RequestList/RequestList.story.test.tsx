import React from 'react';

import { render } from 'design/utils/testing';

import { Loaded } from './RequestList.story';

test('loaded state', () => {
  global.IntersectionObserver = jest.fn(callback => {
    callback(
      [
        {
          // This is the property that triggers the fetch. We need it to be true.
          isIntersecting: true,
          intersectionRatio: null,
          boundingClientRect: null,
          intersectionRect: null,
          rootBounds: null,
          target: null,
          time: null,
        },
      ],
      null
    );
    return {
      observe: jest.fn(),
      unobserve: jest.fn(),
      disconnect: jest.fn(),
      takeRecords: jest.fn(),
      root: null,
      rootMargin: null,
      thresholds: null,
    };
  });
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});
