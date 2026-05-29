const MANILA_TIMEZONE = "Asia/Manila";
const MANILA_OFFSET_MS = 8 * 60 * 60 * 1000;

let manilaSupportChecked = false;
let manilaSupported = false;

export function supportsTimeZone(timeZone: string): boolean {
  try {
    new Intl.DateTimeFormat("en-US", { timeZone }).format(new Date());
    return true;
  } catch {
    return false;
  }
}

export function supportsManilaTimeZone(): boolean {
  if (!manilaSupportChecked) {
    manilaSupported = supportsTimeZone(MANILA_TIMEZONE);
    manilaSupportChecked = true;
  }
  return manilaSupported;
}

function toManilaFallbackDate(input: Date): Date {
  return new Date(input.getTime() + MANILA_OFFSET_MS);
}

export function formatInManila(
  input: Date,
  locale: string,
  options: Intl.DateTimeFormatOptions,
): string {
  if (supportsManilaTimeZone()) {
    return new Intl.DateTimeFormat(locale, {
      ...options,
      timeZone: MANILA_TIMEZONE,
    }).format(input);
  }

  // Fallback: emulate Manila by shifting to UTC+8 and formatting as UTC.
  return new Intl.DateTimeFormat(locale, {
    ...options,
    timeZone: "UTC",
  }).format(toManilaFallbackDate(input));
}

export function toManilaDateOnly(input: Date): string {
  if (supportsManilaTimeZone()) {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone: MANILA_TIMEZONE,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(input);

    const year = parts.find((part) => part.type === "year")?.value ?? "0000";
    const month = parts.find((part) => part.type === "month")?.value ?? "00";
    const day = parts.find((part) => part.type === "day")?.value ?? "00";

    return `${year}-${month}-${day}`;
  }

  const fallback = toManilaFallbackDate(input);
  const year = fallback.getUTCFullYear();
  const month = String(fallback.getUTCMonth() + 1).padStart(2, "0");
  const day = String(fallback.getUTCDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export { MANILA_TIMEZONE };
