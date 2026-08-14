import FieldInput from 'shared/components/FieldInput';
import { Form } from 'shared/components/FileTransfer/FileTransferStateless/CommonElements';

/**
 * Require user to search and hit [enter] to trigger search.
 * This is to keep searching experience consistent for this feature.
 * (other areas of this feature require hitting [enter])
 */
export function SubmitSearchInput({
  defaultValue,
  searchInputName,
  setSearchValue,
  placeholder,
  disabled = false,
}: {
  defaultValue: string;
  searchInputName: string;
  setSearchValue(search: string): void;
  placeholder: string;
  disabled?: boolean;
}) {
  return (
    <Form
      onSubmit={e => {
        e.preventDefault(); // prevent form default
        const formData = new FormData(e.currentTarget);
        const searchValue = formData.get(searchInputName) as string;
        setSearchValue(searchValue.toLocaleLowerCase());
      }}
    >
      <FieldInput
        defaultValue={defaultValue}
        name={searchInputName}
        placeholder={placeholder}
        mb={2}
        disabled={disabled}
      />
      <input type="submit" style={{ display: 'none' }} />
    </Form>
  );
}
