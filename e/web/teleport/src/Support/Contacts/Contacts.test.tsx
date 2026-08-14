import { MemoryRouter, Route, Routes } from 'react-router';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { contactsService } from 'e-teleport/services/contacts';
import {
  ContactType,
  ContactVerification,
} from 'e-teleport/services/contacts/types';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Access } from 'teleport/services/user';
import { defaultAccess } from 'teleport/services/user/makeAcl';

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

beforeEach(() => {
  jest.resetAllMocks();
});

test('fetches and displays contacts', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValueOnce(contacts);

  renderElement(<Contacts clusterId="cluster-id" />);

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();
  expect(await screen.findByText('Security Contacts')).toBeInTheDocument();

  expect(screen.getByDisplayValue('alice@goteleport.com')).toBeInTheDocument();
  expect(screen.getByDisplayValue('bob@goteleport.com')).toBeInTheDocument();

  expect(contactsService.fetchContacts).toHaveBeenCalledTimes(1);
  expect(contactsService.fetchContacts).toHaveBeenCalledWith('cluster-id');
});

test('does not display contacts when access.list is false', async () => {
  renderElement(<Contacts clusterId="cluster-id" />, { list: false });

  await waitFor(() => {
    expect(screen.queryByText('Business Contacts')).not.toBeInTheDocument();
  });
  expect(screen.queryByText('Security Contacts')).not.toBeInTheDocument();
});

test('does not display action buttons when access.create and access.remove are false', async () => {
  jest.spyOn(contactsService, 'fetchContacts').mockResolvedValueOnce(contacts);
  renderElement(<Contacts clusterId="cluster-id" />, {
    create: false,
    remove: false,
    list: true,
  });

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();
  expect(await screen.findByText('alice@goteleport.com')).toBeInTheDocument();
  expect(await screen.findByText('bob@goteleport.com')).toBeInTheDocument();

  // "Invite New" button should not be present
  expect(screen.queryByText('Invite New')).not.toBeInTheDocument();
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

  renderElement(<Contacts clusterId="cluster-id" />);

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();

  // create business contact
  const addButtons = await screen.findAllByText('Invite New');
  await waitFor(() => {
    expect(addButtons[1]).toBeEnabled();
  });
  fireEvent.click(addButtons[1]);

  let emailInputs = screen.getAllByPlaceholderText('mail@example.com');
  // the second email input is the business one (Security contacts appear first)
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
  fireEvent.click(addButtons[0]);

  emailInputs = screen.getAllByPlaceholderText('mail@example.com');
  fireEvent.change(emailInputs[0], {
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

  renderElement(<Contacts clusterId="cluster-id" />);

  expect(
    await screen.findByDisplayValue('alice@goteleport.com')
  ).toBeInTheDocument();

  const deleteButtons = screen.getAllByTestId('delete-contact-btn');
  // alice is a business contact, so the delete button is in the business section (second)

  fireEvent.click(deleteButtons[1]);

  // wait for delete dialog
  expect(
    await screen.findByText(
      'Are you sure you want to delete this business contact?'
    )
  ).toBeInTheDocument();

  // click the confirmation button
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

  await waitFor(() => {
    expect(
      screen.queryByDisplayValue('alice@goteleport.com')
    ).not.toBeInTheDocument();
  });

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

  renderElement(<Contacts clusterId="cluster-id" />);

  expect(await screen.findByText('Business Contacts')).toBeInTheDocument();

  const addButtons = await screen.findAllByText('Invite New');
  await waitFor(() => {
    expect(addButtons[0]).toBeEnabled();
  });
  fireEvent.click(addButtons[0]);

  // Wait for the new draft contact input to appear
  const emailInput = await screen.findByPlaceholderText('mail@example.com');
  fireEvent.change(emailInput, {
    target: { value: 'valid@mail.com' },
  });

  const inviteButton = screen.getByText('Invite');
  fireEvent.click(inviteButton);

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

  renderElement(<Contacts clusterId="cluster-id" />);

  expect(
    await screen.findByDisplayValue('alice@goteleport.com')
  ).toBeInTheDocument();

  const deleteButtons = screen.getAllByTestId('delete-contact-btn');
  fireEvent.click(deleteButtons[1]);

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

  renderElement(<Contacts clusterId="cluster-id" />);

  expect(await screen.findAllByText('Failed to fetch contacts')).toHaveLength(
    2
  );

  expect(contactsService.fetchContacts).toHaveBeenCalled();
});
