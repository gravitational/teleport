export default function formatCents(cents?: number): string {
  if (Number.isInteger(cents)) {
    const value = cents / 100;
    return value.toLocaleString('en-US', {
      style: 'currency',
      currency: 'USD',
    });
  }

  return 'unknown format';
}
