import { useState } from 'react';

import Validation from 'shared/components/Validation';

import { CreateFilters as CreateFiltersComponent } from './CreateFilters';

export default {
  title: 'TeleportE/Integrations/shared/CreateFilters',
};

export function Default() {
  const [filters, setFilters] = useState([]);
  return (
    <Validation>
      {({ validator }) => (
        <CreateFiltersComponent
          filters={filters}
          label="Configure filter with matching value"
          validator={validator}
          onFilterChange={e => {
            setFilters(e);
          }}
          placeholder="Type a value and press enter"
          isDisabled={false}
        />
      )}
    </Validation>
  );
}

export function Disabled() {
  const [filters, setFilters] = useState([
    { label: 'uuidv4-1', value: 'uuidv4-1', invalid: false },
  ]);
  return (
    <Validation>
      {({ validator }) => (
        <CreateFiltersComponent
          filters={filters}
          label="Configure filter with matching value"
          validator={validator}
          onFilterChange={e => {
            setFilters(e);
          }}
          placeholder="Type a value and press enter"
          isDisabled={true}
        />
      )}
    </Validation>
  );
}

export function WithError() {
  const [filters, setFilters] = useState([
    { label: 'uuidv4-1', value: 'uuidv4-1', invalid: false },
    { label: 'uuidv4-2', value: 'uuidv4-2', invalid: true },
    { label: 'uuidv4-3', value: 'uuidv4-3', invalid: false },
  ]);
  return (
    <Validation>
      {({ validator }) => (
        <CreateFiltersComponent
          filters={filters}
          label="Configure filter with matching value"
          validator={validator}
          onFilterChange={e => {
            setFilters(e);
          }}
          placeholder="Type a value and press enter"
          isDisabled={false}
        />
      )}
    </Validation>
  );
}
