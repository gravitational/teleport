import { Delete } from './Delete';

export default {
  title: 'TeleportE/Discover/SAML Application/shared/SamlAppActions/Delete',
};

export const Default = () => {
  return (
    <Delete
      {...props}
      attempt={{ status: 'success', data: null, statusText: '' }}
    />
  );
};

export const Processing = () => {
  return (
    <Delete
      {...props}
      attempt={{ status: 'processing', data: null, statusText: '' }}
    />
  );
};

export const Error = () => {
  return (
    <Delete
      {...props}
      attempt={{
        status: 'error',
        data: null,
        statusText: 'Error while deleting resource',
        error: 'err',
      }}
    />
  );
};

const props = {
  open: true,
  onClose: () => null,
  appName: 'my_app',
  onDelete: () => null,
  attempt: {},
};
