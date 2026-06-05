// Format a Date as a local-time YYYY-MM-DD string. Using toISOString() here
// would convert to UTC and shift the day in non-UTC timezones.
function localISODate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// Inclusive list of YYYY-MM-DD strings between start and end.
export function dateRange(start: string, end: string): string[] {
  const out: string[] = [];
  const s = new Date(start + "T00:00:00");
  const e = new Date(end + "T00:00:00");
  for (let d = s; d <= e; d.setDate(d.getDate() + 1)) {
    out.push(localISODate(d));
  }
  return out;
}

export function shortDate(iso: string): string {
  return new Date(iso + "T00:00:00").toLocaleDateString("ru-RU", {
    weekday: "short",
    day: "numeric",
    month: "short",
  });
}

export function today(): string {
  return localISODate(new Date());
}

export function addDays(iso: string, days: number): string {
  const d = new Date(iso + "T00:00:00");
  d.setDate(d.getDate() + days);
  return localISODate(d);
}
