import { render, screen } from 'design/utils/testing';
import { AuthProviderType } from 'shared/services';

import ConnectorListItem from './ConnectorListItem';

describe('connectorListItem', () => {
  const dummyOnEdit = jest.fn();
  const dummyOnDelete = jest.fn();

  const defaultProps = {
    id: 'dummyId',
    name: 'dummyName',
    kind: 'oidc' as AuthProviderType,
    onEdit: dummyOnEdit,
    onDelete: dummyOnDelete,
    showAuthConnectorsCTA: false,
  };

  const renderComponent = (props = {}) => {
    const mergedProps = { ...defaultProps, ...props };
    return render(<ConnectorListItem {...mergedProps} />);
  };

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('renders properly', () => {
    renderComponent();
    expect(screen.getByText(defaultProps.name)).toBeInTheDocument();
  });

  test('displays correct name and description', () => {
    renderComponent();
    expect(screen.getByText(defaultProps.name)).toBeInTheDocument();
    expect(screen.getByText('OIDC')).toBeInTheDocument();
  });

  test('changes connector type', () => {
    const newKind = 'github';
    renderComponent({ kind: newKind });
    expect(screen.getByText(`GitHub`)).toBeInTheDocument();
  });
});
