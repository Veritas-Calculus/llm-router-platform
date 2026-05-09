export type CsvRow = Record<string, string>;

export function parseCsv(text: string): CsvRow[] {
  const records = parseCsvRecords(text.replace(/^\uFEFF/, ''));
  const nonEmpty = records.filter((record) => record.some((cell) => cell.trim() !== ''));
  if (nonEmpty.length === 0) return [];

  const headers = nonEmpty[0].map(normalizeHeader);
  return nonEmpty.slice(1).map((record) => {
    const row: CsvRow = {};
    headers.forEach((header, index) => {
      if (header) row[header] = (record[index] || '').trim();
    });
    return row;
  });
}

export function downloadCsvTemplate(filename: string, headers: string[], rows: string[][]) {
  const content = [
    headers.map(escapeCsvCell).join(','),
    ...rows.map((row) => row.map(escapeCsvCell).join(',')),
  ].join('\n');
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function csvBool(value: string | undefined, fallback = true): boolean {
  if (!value) return fallback;
  const normalized = value.trim().toLowerCase();
  if (['true', '1', 'yes', 'y', 'active', 'enabled'].includes(normalized)) return true;
  if (['false', '0', 'no', 'n', 'inactive', 'disabled'].includes(normalized)) return false;
  return fallback;
}

function parseCsvRecords(text: string): string[][] {
  const records: string[][] = [];
  let record: string[] = [];
  let cell = '';
  let inQuotes = false;

  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    const next = text[i + 1];

    if (char === '"') {
      if (inQuotes && next === '"') {
        cell += '"';
        i++;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }

    if (char === ',' && !inQuotes) {
      record.push(cell);
      cell = '';
      continue;
    }

    if ((char === '\n' || char === '\r') && !inQuotes) {
      if (char === '\r' && next === '\n') i++;
      record.push(cell);
      records.push(record);
      record = [];
      cell = '';
      continue;
    }

    cell += char;
  }

  record.push(cell);
  records.push(record);
  return records;
}

function normalizeHeader(value: string): string {
  return value.trim().toLowerCase().replace(/[\s-]+/g, '_');
}

function escapeCsvCell(value: string): string {
  if (!/[",\n\r]/.test(value)) return value;
  return `"${value.replace(/"/g, '""')}"`;
}
