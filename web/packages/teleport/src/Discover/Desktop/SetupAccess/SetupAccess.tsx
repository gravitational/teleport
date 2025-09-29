import Box from "design/Box";
import { useEffect, useState } from "react";
import { Option, SelectCreatable } from "teleport/Discover/Shared/SelectCreatable";
import {
  SetupAccessWrapper,
  type State,
  useUserTraits
} from "teleport/Discover/Shared/SetupAccess";

export default function Container() {
  const state = useUserTraits();
  return <SetupAccess {...state} />;
}

function SetupAccess(props: State) {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    agentMeta,
    ...restOfProps
  } = props;
  const [loginInputValue, setLoginInputValue] = useState('');
  const [selectedLogins, setSelectedLogins] = useState<Option[]>([]);
  
  useEffect(() => {
    if (props.attempt.status === 'success') {
      setSelectedLogins(initSelectedOptions('windowsLogins'));
    }
  }, [props.attempt.status, initSelectedOptions]);

  function handleOnProceed() {
    onProceed({ windowsLogins: selectedLogins }, 1);
  }

  function handleLoginKeyDown(event: React.KeyboardEvent) {
    if (!loginInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setSelectedLogins([
          ...selectedLogins,
          { value: loginInputValue, label: loginInputValue },
        ]);
        setLoginInputValue('');
        event.preventDefault();
    }
  }

  const hasTraits = selectedLogins.length > 0;
  const canAddTraits = !props.isSsoUser && props.canEditUser;
  const headerSubtitle =
    'Select or create the OS users you will use to connect to the desktop.';

  return (
    <SetupAccessWrapper
      {...restOfProps}
      headerSubtitle={headerSubtitle}
      traitKind="Desktop"
      traitDescription="Desktop access traits"
      hasTraits={hasTraits}
      onProceed={handleOnProceed}
    >
      <Box mb={2}>
        OS Users
        <SelectCreatable
          inputValue={loginInputValue}
          isClearable={selectedLogins.some(v => !v.isFixed)}
          onInputChange={setLoginInputValue}
          onKeyDown={handleLoginKeyDown}
          placeholder="Start typing OS users and press enter"
          value={selectedLogins}
          isDisabled={!canAddTraits}
          onChange={(value, action) => {
            if (action.action === 'clear') {
              setSelectedLogins(getFixedOptions('windowsLogins'));
            } else {
              setSelectedLogins(value || []);
            }
          }}
          options={getSelectableOptions('windowsLogins')}
        />
      </Box>
    </SetupAccessWrapper>
  )
}