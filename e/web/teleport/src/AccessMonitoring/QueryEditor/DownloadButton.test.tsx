import { fireEvent, render, screen } from 'design/utils/testing';
import * as saveOnDiskUtils from 'shared/utils/saveOnDisk';

import DownloadButton from './DownloadButton';

describe('DownloadButton', () => {
  const mockSaveOnDisk = jest
    .spyOn(saveOnDiskUtils, 'saveOnDisk')
    .mockImplementation(() => {});

  beforeEach(() => {
    mockSaveOnDisk.mockClear();
  });

  it('generates correct CSV for normal strings', () => {
    const header = ['Username', 'Login Count', 'IP'];
    const rows = [
      ['alice', '30', '12.34.56.78'],
      ['bob', '25', '12.34.56.78'],
    ];

    render(<DownloadButton header={header} rows={rows} resultId="123" />);
    const button = screen.getByRole('button', { name: /download csv/i });

    fireEvent.click(button);

    expect(mockSaveOnDisk).toHaveBeenCalledWith(
      `Username,Login Count,IP
alice,30,12.34.56.78
bob,25,12.34.56.78`,
      'result-123.csv',
      'text/csv'
    );
  });

  it('generates correct CSV for strings with quotes and commas', () => {
    const header = ['Username', 'Comment', 'Cluster'];
    const rows = [
      ['alice', 'A "comment",', 'weyland-yutani'],
      ['bob', 'N/A', 'weyland-yutani'],
      ['charlie', 'Some msg, with a comma.', 'weyland-yutani'],
    ];

    render(<DownloadButton header={header} rows={rows} resultId="456" />);
    const button = screen.getByRole('button', { name: /download csv/i });

    fireEvent.click(button);

    expect(mockSaveOnDisk).toHaveBeenCalledWith(
      `Username,Comment,Cluster
alice,"A ""comment"",",weyland-yutani
bob,N/A,weyland-yutani
charlie,"Some msg, with a comma.",weyland-yutani`,
      'result-456.csv',
      'text/csv'
    );
  });

  it('disables the button when header or rows are empty', () => {
    const { rerender } = render(
      <DownloadButton header={[]} rows={[]} resultId="123" />
    );
    let button = screen.getByRole('button', { name: /download csv/i });
    expect(button).toBeDisabled();

    rerender(<DownloadButton header={['Name']} rows={[]} resultId="123" />);
    button = screen.getByRole('button', { name: /download csv/i });
    expect(button).toBeDisabled();

    rerender(<DownloadButton header={[]} rows={[['Alice']]} resultId="123" />);
    button = screen.getByRole('button', { name: /download csv/i });
    expect(button).toBeDisabled();
  });
});
