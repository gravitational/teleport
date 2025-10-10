import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';

import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { CustomAccessList } from './CreateWizard/CustomAccessList';

export const CreateAccessListWithProvider = () => (
  <AccessListManagementContextProvider>
    <CreateAccessListContextProvider>
      <CreateAccessList />
    </CreateAccessListContextProvider>
  </AccessListManagementContextProvider>
);

export function CreateAccessList() {
  return <CustomAccessList />;
}
