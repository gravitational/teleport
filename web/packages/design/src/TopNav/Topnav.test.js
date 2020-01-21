import React from 'react';
import { render } from '@testing-library/react';
import { TopNavComponent } from './TopNav.story';
import ThemeProvider from 'design/ThemeProvider';
import theme from 'design/theme';

// TODO: find out how to setup a default rendering container (with ThemeProvider)
describe('Design/TopNav', () => {
  it('should render', () => {
    const { container } = render(
      <ThemeProvider theme={theme}>
        <TopNavComponent />
      </ThemeProvider>
    );

    expect(container.querySelectorAll('nav')).toHaveLength(1);
    expect(container.querySelectorAll('button')).toHaveLength(3);
  });
});
