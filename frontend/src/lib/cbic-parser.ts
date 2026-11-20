export interface ExtractedCurrencyRate {
  currencyCode: string;
  currencyName: string;
  currencyUnit: number;
  rateImport: number;
  rateExport: number;
  effectiveRate: number; // Statutory rate per 1 unit for duty calculation (rateImport / currencyUnit)
}

export interface ParsedCBICNotification {
  notificationNumber: string;
  effectiveFrom: string; // YYYY-MM-DD
  rates: ExtractedCurrencyRate[];
  rawText: string;
  warnings?: string[];
}

const KNOWN_CURRENCY_NAMES: Record<string, string> = {
  AED: 'UAE Dirham',
  AUD: 'Australian Dollar',
  BHD: 'Bahraini Dinar',
  CAD: 'Canadian Dollar',
  CHF: 'Swiss Franc',
  CNY: 'Chinese Yuan',
  DKK: 'Danish Kroner',
  EUR: 'EURO',
  GBP: 'Pound Sterling',
  HKD: 'Hong Kong Dollar',
  JPY: 'Japanese Yen',
  KRW: 'Korean won',
  KWD: 'Kuwaiti Dinar',
  NOK: 'Norwegian Kroner',
  NZD: 'New Zealand Dollar',
  QAR: 'Qatari Riyal',
  SAR: 'Saudi Arabian Riyal',
  SEK: 'Swedish Kroner',
  SGD: 'Singapore Dollar',
  TRY: 'Turkish Lira',
  USD: 'US Dollar',
  ZAR: 'South African Rand',
};

/**
 * Extracts text from a PDF ArrayBuffer or Uint8Array, preserving line breaks.
 * Lazily loads pdfjs-dist on demand to prevent heavy initial bundle bloat.
 */
export async function extractTextFromPdf(data: ArrayBuffer | Uint8Array): Promise<string> {
  const pdfjsLib = await import('pdfjs-dist');

  if (typeof window !== 'undefined' && !pdfjsLib.GlobalWorkerOptions.workerSrc) {
    try {
      pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
        'pdfjs-dist/build/pdf.worker.min.mjs',
        import.meta.url,
      ).toString();
    } catch {
      // Ignore URL constructor failure in non-browser/test environments
    }
  }

  const loadingTask = pdfjsLib.getDocument({
    data,
    useSystemFonts: true,
  });
  const doc = await loadingTask.promise;
  const pageTexts: string[] = [];

  for (let i = 1; i <= doc.numPages; i++) {
    const page = await doc.getPage(i);
    const content = await page.getTextContent();
    let pageText = '';
    let lastY: number | null = null;

    for (const rawItem of content.items) {
      if ('str' in rawItem) {
        const item = rawItem as { str: string; transform?: number[]; hasEOL?: boolean };
        const y: number | null =
          item.transform && item.transform.length >= 6 && typeof item.transform[5] === 'number'
            ? item.transform[5]
            : null;

        if (lastY !== null && y !== null && Math.abs(y - lastY) > 4) {
          pageText += '\n';
        } else if (pageText.length > 0 && !pageText.endsWith('\n') && !pageText.endsWith(' ')) {
          pageText += ' ';
        }

        pageText += item.str;
        if (item.hasEOL) {
          pageText += '\n';
        }
        if (y !== null) {
          lastY = y;
        }
      }
    }
    pageTexts.push(pageText);
  }

  return pageTexts.join('\n');
}

/**
 * Parses raw text from a CBIC Customs Exchange Rate notification into structured notification data.
 */
export function parseCBICNotificationText(text: string): ParsedCBICNotification {
  const warnings: string[] = [];

  // 1. Extract Notification Number (e.g., "28/2026", "Notification No.: 28/2026", "28/2026-Customs (N.T.)")
  let notificationNumber = '';
  const notifMatch = text.match(/(?:Exchange\s*Rate\s*)?Notification\s*(?:No\.?|Number)?[:\s]*([0-9]+\s*\/\s*[0-9]+(?:-[A-Za-z\s\(\)\.]+)?)/i);
  if (notifMatch?.[1]) {
    const rawNum = notifMatch[1].trim();
    notificationNumber = rawNum.toLowerCase().startsWith('notification')
      ? rawNum
      : `Notification No. ${rawNum}`;
  } else {
    // Fallback: look for patterns like "No. 28/2026" or "28/2026"
    const fallbackMatch = text.match(/\b([0-9]{1,3}\/[0-9]{4})\b/);
    if (fallbackMatch?.[1]) {
      notificationNumber = `Notification No. ${fallbackMatch[1]}`;
    } else {
      notificationNumber = `CBIC/${new Date().getFullYear()}`;
      warnings.push('Could not automatically determine notification number from document.');
    }
  }

  // 2. Extract Effective Date (w.e.f DD-MM-YYYY or DD/MM/YYYY or YYYY-MM-DD)
  let effectiveFrom = '';
  const wefMatch = text.match(/w\.?e\.?f\.?[:\s]*([0-9]{1,2}[-\/\.][0-9]{1,2}[-\/\.][0-9]{4}|[0-9]{4}[-\/\.][0-9]{1,2}[-\/\.][0-9]{1,2})/i);
  if (wefMatch?.[1]) {
    const rawDate = wefMatch[1].trim();
    if (rawDate.includes('-') || rawDate.includes('/') || rawDate.includes('.')) {
      const parts = rawDate.split(/[-\/\.]/);
      const p0 = parts[0] ?? '';
      const p1 = parts[1] ?? '';
      const p2 = parts[2] ?? '';
      if (p0.length === 4 && p1 && p2) {
        // YYYY-MM-DD
        effectiveFrom = `${p0}-${p1.padStart(2, '0')}-${p2.padStart(2, '0')}`;
      } else if (p2.length === 4 && p0 && p1) {
        // DD-MM-YYYY
        effectiveFrom = `${p2}-${p1.padStart(2, '0')}-${p0.padStart(2, '0')}`;
      }
    }
  }

  if (!effectiveFrom) {
    effectiveFrom = new Date().toISOString().slice(0, 10);
    warnings.push('Could not determine effective date (w.e.f); defaulted to current date.');
  }

  // 3. Extract Currencies
  const rates: ExtractedCurrencyRate[] = [];
  const lines = text.split(/\r?\n/);
  const seenCurrencies = new Set<string>();

  // Pattern A: Tabular rows matching: "CODE  NAME  UNIT  RATE_IMPORT  RATE_EXPORT"
  // Example: "AED UAE Dirham 1.0 26.95 25.4"
  // Example: "JPY Japanese Yen 100.0 61.6 59.75"
  // Example: "TRY Turkish Lira 1.0 2 1.9"
  const tableRegex = /^\s*([A-Z]{3})\s+(.*?)\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s*$/;

  // Pattern B: Schedule-based row where Unit is implied or separate
  // Example: "1. US Dollar USD 84.15 82.50"
  const scheduleRegex = /(?:^|\s)([A-Z]{3})\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)(?:\s|$)/;

  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line) continue;

    // Check Pattern A
    const matchA = line.match(tableRegex);
    if (matchA && matchA[1] && matchA[3] && matchA[4] && matchA[5]) {
      const code = matchA[1].toUpperCase();
      if (KNOWN_CURRENCY_NAMES[code] || /^[A-Z]{3}$/.test(code)) {
        const rawName = (matchA[2] ?? '').trim();
        const name = rawName || KNOWN_CURRENCY_NAMES[code] || code;
        const unit = parseFloat(matchA[3]) || 1.0;
        const rateImport = parseFloat(matchA[4]);
        const rateExport = parseFloat(matchA[5]);
        const effectiveRate = unit > 0 ? Number((rateImport / unit).toFixed(6)) : rateImport;

        if (!seenCurrencies.has(code) && rateImport > 0) {
          rates.push({
            currencyCode: code,
            currencyName: name,
            currencyUnit: unit,
            rateImport,
            rateExport,
            effectiveRate,
          });
          seenCurrencies.add(code);
        }
        continue;
      }
    }

    // Check Pattern B across line tokens
    const matchB = line.match(scheduleRegex);
    if (matchB && matchB[1] && matchB[2] && matchB[3]) {
      const code = matchB[1].toUpperCase();
      if (KNOWN_CURRENCY_NAMES[code] && !seenCurrencies.has(code)) {
        const rateImport = parseFloat(matchB[2]);
        const rateExport = parseFloat(matchB[3]);
        const unit = code === 'JPY' || code === 'KRW' ? 100.0 : 1.0;
        const effectiveRate = Number((rateImport / unit).toFixed(6));

        rates.push({
          currencyCode: code,
          currencyName: KNOWN_CURRENCY_NAMES[code] ?? code,
          currencyUnit: unit,
          rateImport,
          rateExport,
          effectiveRate,
        });
        seenCurrencies.add(code);
      }
    }
  }

  // If tableRegex didn't catch multiline or concatenated token flows, scan by known currency tokens
  if (rates.length === 0) {
    for (const [code, name] of Object.entries(KNOWN_CURRENCY_NAMES)) {
      if (seenCurrencies.has(code)) continue;
      // Search for code followed by numbers
      const codeRegex = new RegExp(`\\b${code}\\b[^\\d]*([0-9]+(?:\\.[0-9]+)?)[^\\d]+([0-9]+(?:\\.[0-9]+)?)[^\\d]+([0-9]+(?:\\.[0-9]+)?)`, 'i');
      const m = text.match(codeRegex);
      if (m && m[1] && m[2] && m[3]) {
        const unitCandidate = parseFloat(m[1]);
        const importCandidate = parseFloat(m[2]);
        const exportCandidate = parseFloat(m[3]);
        let unit = 1.0;
        let rateImport = importCandidate;
        let rateExport = exportCandidate;

        if (unitCandidate === 100 || unitCandidate === 1) {
          unit = unitCandidate;
          rateImport = importCandidate;
          rateExport = exportCandidate;
        } else {
          // unit might not be printed, numbers might be import and export
          rateImport = unitCandidate;
          rateExport = importCandidate;
          unit = code === 'JPY' || code === 'KRW' ? 100.0 : 1.0;
        }

        const effectiveRate = Number((rateImport / unit).toFixed(6));
        rates.push({
          currencyCode: code,
          currencyName: name,
          currencyUnit: unit,
          rateImport,
          rateExport,
          effectiveRate,
        });
        seenCurrencies.add(code);
      }
    }
  }

  // Sort rates alphabetically by currency code for clean presentation
  rates.sort((a, b) => a.currencyCode.localeCompare(b.currencyCode));

  if (rates.length === 0) {
    warnings.push('No currency exchange rate rows were identified in the uploaded document.');
  }

  return {
    notificationNumber,
    effectiveFrom,
    rates,
    rawText: text,
    warnings: warnings.length > 0 ? warnings : undefined,
  };
}
