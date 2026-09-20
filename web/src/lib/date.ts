/**
 * Formats a protobuf timestamp with seconds into a localized date string.
 */
export function formatTimestamp(ts?: { seconds: bigint }): string {
  if (!ts || !ts.seconds) return '';
  const d = new Date(Number(ts.seconds) * 1000);
  return d.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Formats an ISO-8601 calendar date (YYYY-MM-DD) into a human-readable string.
 */
export function formatBirthDate(dateStr?: string, estimated?: boolean): string {
  if (!dateStr) return 'Unknown';
  const parts = dateStr.split('-');
  if (parts.length !== 3) return dateStr;
  const date = new Date(parseInt(parts[0], 10), parseInt(parts[1], 10) - 1, parseInt(parts[2], 10));
  const formatted = date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  return estimated ? `${formatted} (estimated)` : formatted;
}

/**
 * Calculates human-readable age from a calendar birth date (YYYY-MM-DD).
 */
export function calculateAge(dateStr?: string): string {
  if (!dateStr) return '';
  const parts = dateStr.split('-');
  if (parts.length !== 3) return '';
  const birth = new Date(parseInt(parts[0], 10), parseInt(parts[1], 10) - 1, parseInt(parts[2], 10));
  const now = new Date();

  let years = now.getFullYear() - birth.getFullYear();
  let months = now.getMonth() - birth.getMonth();
  if (now.getDate() < birth.getDate()) {
    months--;
  }
  if (months < 0) {
    years--;
    months += 12;
  }

  if (years < 0) return 'Just born';
  if (years === 0) {
    if (months <= 0) return '< 1 month old';
    if (months === 1) return '1 month old';
    return `${months} months old`;
  }
  if (years === 1) {
    if (months === 0) return '1 year old';
    return `1 year, ${months} ${months === 1 ? 'month' : 'months'} old`;
  }
  return `${years} years old`;
}
