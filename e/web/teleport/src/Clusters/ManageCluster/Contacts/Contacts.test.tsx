import { MemoryRouter, Route } from 'react-router-dom';
import { render, waitFor, screen, fireEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Access } from 'teleport/services/user';
import { defaultAccess } from 'teleport/services/user/makeAcl';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  ContactType,
  ContactVerification,
} from 'e-teleport/services/contacts/types';
import { contactsService } from 'e-teleport/services/contacts';

import { Contacts } from './Contacts';
import { contacts } from './fixtures';

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
      <Route path="/clusters/:clusterId">
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>{element}</ContextProvider>
        </ContentMinWidth>
      </Route>
    </MemoryRouter>
  );
}

beforeEach(() => {
  jest.resetAllMocks();
});

test('fetches and displays contacts', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValueOnce(contacts);

  renderElement(<Contacts />);

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();
  expect(await screen.findByText('Security Contacts')).toBeInTheDocument();

  expect(screen.getByDisplayValue('alice@goteleport.com')).toBeInTheDocument();
  expect(screen.getByDisplayValue('bob@goteleport.com')).toBeInTheDocument();

  expect(contactsService.fetchContacts).toHaveBeenCalledTimes(1);
  expect(contactsService.fetchContacts).toHaveBeenCalledWith('cluster-id');
});

test('does not display contacts when access.list is false', async () => {
  renderElement(<Contacts />, { list: false });

  await waitFor(() => {
    expect(screen.queryByText('Business Contacts')).not.toBeInTheDocument();
  });
  expect(screen.queryByText('Security Contacts')).not.toBeInTheDocument();
});

test('does not display action buttons when access.create and access.remove are false', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValueOnce(contacts);
  renderElement(<Contacts />, {
    create: false,
    remove: false,
    list: true,
  });

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();
  expect(await screen.findByText('alice@goteleport.com')).toBeInTheDocument();
  expect(await screen.findByText('bob@goteleport.com')).toBeInTheDocument();

  // "Add New" button should not be present
  expect(screen.queryByText('Add New')).not.toBeInTheDocument();
});

test('allows adding a new contact', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValue([]);
  jest.spyOn(contactsService, 'createContact').mockResolvedValue({
    name: '',
    accountId: 'account789',
    verifyToken: 'token789',
    email: 'newcontact@example.com',
    contactType: ContactType.Business,
    verification: ContactVerification.Pending,
  });

  renderElement(<Contacts />);

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();

  // create business contact
  let addButtons = screen.getAllByText('Add New');
  fireEvent.click(addButtons[0]);

  let emailInputs = screen.getAllByPlaceholderText('mail@example.com');
  // the first email input is the business one
  fireEvent.change(emailInputs[0], {
    target: { value: 'newcontact-business@example.com' },
  });
  let inviteButtons = screen.getAllByText('Invite');
  fireEvent.click(inviteButtons[0]);

  await waitFor(() => {
    expect(
      screen.getByDisplayValue('newcontact-business@example.com')
    ).toBeInTheDocument();
  });

  expect(contactsService.createContact).toHaveBeenCalledWith(
    'cluster-id',
    'newcontact-business@example.com',
    ContactType.Business
  );

  // create security contact
  fireEvent.click(addButtons[1]); // add new

  emailInputs = screen.getAllByPlaceholderText('mail@example.com');
  fireEvent.change(emailInputs[1], {
    target: { value: 'newcontact-security@example.com' },
  });
  inviteButtons = screen.getAllByText('Invite');
  fireEvent.click(inviteButtons[0]);

  await waitFor(() => {
    expect(
      screen.getByDisplayValue('newcontact-security@example.com')
    ).toBeInTheDocument();
  });

  expect(contactsService.createContact).toHaveBeenCalledWith(
    'cluster-id',
    'newcontact-security@example.com',
    ContactType.Security
  );
});

test('allows deleting a contact', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValue(contacts);
  jest.spyOn(contactsService, 'deleteContact').mockResolvedValueOnce();

  renderElement(<Contacts />);

  expect(
    await screen.findByDisplayValue('alice@goteleport.com')
  ).toBeInTheDocument();

  const deleteButtons = screen.getAllByText('Delete');
  // the first delete button is alice's

  fireEvent.click(deleteButtons[0]);

  // wait for delete dialog
  expect(
    screen.getByText('Are you sure you want to delete this business contact?')
  ).toBeInTheDocument();

  // click the confirmation button
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

  expect(
    screen.queryByDisplayValue('alice@example.com')
  ).not.toBeInTheDocument();

  expect(contactsService.deleteContact).toHaveBeenCalledWith(
    'cluster-id',
    'token123',
    ContactType.Business
  );

  // assert that bob is still on the screen
  expect(
    await screen.findByDisplayValue('bob@goteleport.com')
  ).toBeInTheDocument();
});

test('handles create contact failure', async () => {
  const mockContacts = [];

  jest
    .spyOn(contactsService, 'fetchContacts')
    .mockResolvedValueOnce(mockContacts);
  jest
    .spyOn(contactsService, 'createContact')
    .mockRejectedValueOnce(new Error('Failed to create contact'));

  renderElement(<Contacts />);

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();

  const addButtons = screen.getAllByText('Add New');
  fireEvent.click(addButtons[0]);

  const emailInputs = screen.getAllByPlaceholderText('mail@example.com');
  fireEvent.change(emailInputs[emailInputs.length - 1], {
    target: { value: 'valid@mail.com' },
  });

  const inviteButtons = screen.getAllByText('Invite');
  fireEvent.click(inviteButtons[inviteButtons.length - 1]);

  // assert that the error returned by the API shows up on screen
  expect(
    await screen.findByText('Failed to create contact')
  ).toBeInTheDocument();

  expect(contactsService.createContact).toHaveBeenCalled();
});

test('handles delete contact failure', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValueOnce(contacts);
  jest
    .spyOn(contactsService, 'deleteContact')
    .mockRejectedValueOnce(new Error('Failed to delete contact'));

  renderElement(<Contacts />);

  expect(
    await screen.findByDisplayValue('alice@goteleport.com')
  ).toBeInTheDocument();

  const deleteButtons = screen.getAllByText('Delete');
  fireEvent.click(deleteButtons[0]);

  // wait for delete dialog
  expect(
    await screen.findByText(
      'Are you sure you want to delete this business contact?'
    )
  ).toBeInTheDocument();

  // click the confirmation button
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

  // assert that the error returned by the API shows up on screen
  expect(
    await screen.findByText('Failed to delete contact')
  ).toBeInTheDocument();

  expect(contactsService.deleteContact).toHaveBeenCalled();
});

test('shows error state when fetching contacts fails', async () => {
  jest
    .spyOn(contactsService, 'fetchContacts')
    .mockRejectedValueOnce(new Error('Failed to fetch contacts'));

  renderElement(<Contacts />);

  expect(await screen.findAllByText('Failed to fetch contacts')).toHaveLength(
    2
  );

  expect(contactsService.fetchContacts).toHaveBeenCalled();
});
