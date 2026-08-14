import { Button } from 'design/Button';
import { saveOnDisk } from 'shared/utils/saveOnDisk';

interface DownloadButtonProps {
  header: string[];
  rows: string[][];
  resultId: string;
}

const escapeCSVValue = (value: string): string => {
  if (!value?.trim()) {
    return '';
  }
  if (value.includes(',') || value.includes('"')) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
};

const constructCSV = (header: string[], rows: string[][]) => {
  const csv = [header.map(escapeCSVValue).join(',')];
  rows.forEach(row => csv.push(row.map(escapeCSVValue).join(',')));
  return csv.join('\n');
};

export default function DownloadButton({
  header,
  rows,
  resultId,
}: DownloadButtonProps) {
  return (
    <Button
      size="medium"
      disabled={!rows.length || !header.length}
      onClick={() => {
        saveOnDisk(
          constructCSV(header, rows),
          `result-${resultId}.csv`,
          'text/csv'
        );
      }}
    >
      Download CSV
    </Button>
  );
}
