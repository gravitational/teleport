import { MemoryRouter, Route, Routes } from 'react-router';

import { fireEvent, render, screen } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  ContactType,
  ContactVerification,
} from 'e-teleport/services/contacts/types';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Access } from 'teleport/services/user';
import { defaultAccess } from 'teleport/services/user/makeAcl';

import {
  ContactEmails,
  ContactEmailsProps,
  FormContact,
} from './ContactEmails';

const contacts: FormContact[] = [
  {
    verifyToken: '1',
    email: '1@example.com',
    accountId: 'acc-id',
    contactType: ContactType.Business,
    verification: ContactVerification.Verified,
    name: 'verified',
    draft: false,
    recentlyInvited: false,
  },
  {
    verifyToken: '2',
    email: '2@example.com',
    accountId: 'acc-id',
    contactType: ContactType.Business,
    verification: ContactVerification.Pending,
    name: 'pending',
    draft: false,
    recentlyInvited: false,
  },
  {
    verifyToken: '3',
    email: '3@example.com',
    accountId: 'acc-id',
    contactType: ContactType.Business,
    verification: ContactVerification.Expired,
    name: 'expired',
    draft: false,
    recentlyInvited: false,
  },
];

function renderElement(element, access?: Partial<Access>) {
  const ctx = createTeleportContextE();
  ctx.storeUser.getContactsAccess = () =>
    access
      ? { ...defaultAccess, ...access }
      : {
          create: true,
          edit: true,
          list: true,
          read: true,
          remove: true,
        };

  return render(
    <MemoryRouter initialEntries={[`/clusters/cluster-id`]}>
      <Routes>
        <Route
          path="/clusters/:clusterId"
          element={
            <InfoGuidePanelProvider>
              <ContentMinWidth>
                <ContextProvider ctx={ctx}>{element}</ContextProvider>
              </ContentMinWidth>
            </InfoGuidePanelProvider>
          }
        />
      </Routes>
    </MemoryRouter>
  );
}

const mockOnChange = jest.fn();
const mockOnNewContact = jest.fn();
const mockOnDelete = jest.fn();
const mockOnInvite = jest.fn();

const defaultProps: ContactEmailsProps = {
  contacts: contacts,
  maxContacts: 3,
  onChange: mockOnChange,
  onNewContact: mockOnNewContact,
  onDelete: mockOnDelete,
  onInvite: mockOnInvite,
  writePermissions: true,
  listAttempt: { status: '', statusText: '', data: null },
  deleteAttempt: { status: '', statusText: '', data: null },
  createAttempt: { status: '', statusText: '', data: null },
};

beforeEach(() => {
  jest.resetAllMocks();
});

test('displays error messages when operations fail', () => {
  // create failed
  renderElement(
    <ContactEmails
      {...defaultProps}
      createAttempt={{
        status: 'error',
        statusText: 'Create failed',
        error: 'error',
        data: null,
      }}
    />
  );
  expect(screen.getByText('Create failed')).toBeInTheDocument();

  // delete failed
  renderElement(
    <ContactEmails
      {...defaultProps}
      deleteAttempt={{
        status: 'error',
        statusText: 'Delete failed',
        error: 'error',
        data: null,
      }}
    />
  );
  expect(screen.getByText('Delete failed')).toBeInTheDocument();
});

test('disables Add New button when max contacts reached', async () => {
  renderElement(
    <ContactEmails {...defaultProps} maxContacts={contacts.length} />
  );

  const addButton = screen.getByRole('button', { name: /Invite New/i });
  expect(addButton).toBeDisabled();
});

test('disables input fields and buttons when processing', () => {
  renderElement(
    <ContactEmails
      {...defaultProps}
      listAttempt={{ status: 'success', statusText: '', data: [] }}
      deleteAttempt={{ status: 'processing', data: null, statusText: '' }}
    />
  );

  const emailInput = screen.getByDisplayValue(contacts[0].email);
  expect(emailInput).toBeDisabled();

  const deleteButtons = screen.getAllByTestId('delete-contact-btn');
  deleteButtons.forEach(button => {
    expect(button).toBeDisabled();
  });
});

test('displays the correct verification state badges', () => {
  renderElement(<ContactEmails {...defaultProps} />);

  expect(
    screen.getByTestId('verification-state-badge-active')
  ).toHaveTextContent('active');
  expect(
    screen.getByTestId('verification-state-badge-pending')
  ).toHaveTextContent('pending');
  expect(
    screen.getByTestId('verification-state-badge-expired')
  ).toHaveTextContent('expired');
});

test('disables Delete button when only one verified contact exists', () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      contactType: ContactType.Business,
      verifyToken: '1',
      email: 'verified@example.com',
      draft: false,
      recentlyInvited: false,
      verification: ContactVerification.Verified,
    },
  ];

  renderElement(<ContactEmails {...defaultProps} contacts={contacts} />);

  const deleteButton = screen.getByTestId('delete-contact-btn');
  expect(deleteButton).toBeDisabled();
});

test('enables Delete button when multiple verified contacts exist', () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '1',
      email: 'verified1@example.com',
      draft: false,
      recentlyInvited: false,
      verification: ContactVerification.Verified,
      contactType: ContactType.Business,
    },
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '2',
      email: 'verified2@example.com',
      draft: false,
      recentlyInvited: false,
      verification: ContactVerification.Verified,
      contactType: ContactType.Business,
    },
  ];

  renderElement(<ContactEmails {...defaultProps} contacts={contacts} />);

  const deleteButtons = screen.getAllByTestId('delete-contact-btn');
  deleteButtons.forEach(button => {
    expect(button).toBeEnabled();
  });
});

test('displays invitation sent message when recently invited', () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '1',
      email: 'newinvite@example.com',
      draft: false,
      recentlyInvited: true,
      verification: ContactVerification.Pending,
      contactType: ContactType.Business,
    },
  ];

  renderElement(<ContactEmails {...defaultProps} contacts={contacts} />);

  expect(
    screen.getByText(
      /Invitation sent successfully and is pending verification. Invitation is valid for 14 days/
    )
  ).toBeInTheDocument();
});

test('validates email before inviting a new contact', async () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '1',
      email: 'invalid-email',
      draft: true,
      recentlyInvited: false,
      verification: ContactVerification.Pending,
      contactType: ContactType.Business,
    },
  ];

  renderElement(<ContactEmails {...defaultProps} contacts={contacts} />);

  const inviteButton = screen.getByText('Invite');
  fireEvent.click(inviteButton);

  expect(mockOnInvite).not.toHaveBeenCalled();
  expect(screen.getByText(/Email Address.*is invalid/i)).toBeInTheDocument();
});

test('calls onInvite when inviting a new contact', async () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '1',
      email: 'new@example.com',
      draft: true,
      recentlyInvited: false,
      verification: ContactVerification.Pending,
      contactType: ContactType.Business,
    },
  ];

  renderElement(<ContactEmails {...defaultProps} contacts={contacts} />);

  const inviteButton = screen.getByText('Invite');
  fireEvent.click(inviteButton);

  expect(mockOnInvite).toHaveBeenCalledWith('1');
});

test('calls onDelete when deleting a contact', () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '1',
      email: 'user@example.com',
      draft: false,
      recentlyInvited: false,
      verification: ContactVerification.Pending,
      contactType: ContactType.Business,
    },
  ];

  renderElement(<ContactEmails {...defaultProps} contacts={contacts} />);

  const deleteButton = screen.getByTestId('delete-contact-btn');
  fireEvent.click(deleteButton);

  expect(mockOnDelete).toHaveBeenCalledWith('1');
});

test('does not render Add New button when writePermissions is false', () => {
  renderElement(<ContactEmails {...defaultProps} writePermissions={false} />);

  expect(
    screen.queryByRole('button', { name: /Invite New/i })
  ).not.toBeInTheDocument();
});

test('does not render Invite and Delete buttons when writePermissions is false', () => {
  const contacts: FormContact[] = [
    {
      accountId: 'acc-id',
      name: 'name',
      verifyToken: '1',
      email: 'user@example.com',
      draft: false,
      recentlyInvited: false,
      verification: ContactVerification.Verified,
      contactType: ContactType.Business,
    },
  ];

  renderElement(
    <ContactEmails
      {...defaultProps}
      contacts={contacts}
      writePermissions={false}
    />
  );

  expect(screen.queryByText('Invite')).not.toBeInTheDocument();
  expect(screen.queryByTestId('delete-contact-btn')).not.toBeInTheDocument();
});
