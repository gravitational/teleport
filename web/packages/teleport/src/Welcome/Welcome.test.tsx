// Try this minimal test first to see if basic routing works
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

// Simple test component
const TestComponent = () => <div>Test Component</div>;

describe('React Router v6 Debug', () => {
  it('should render a simple route', () => {
    render(
      <MemoryRouter
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        initialEntries={['/test']}
      >
        <Routes>
          <Route path="/test" element={<TestComponent />} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Test Component')).toBeInTheDocument();
  });
});
