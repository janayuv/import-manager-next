import { describe, expect, it } from 'vitest';
import { parseCBICNotificationText } from './cbic-parser';

const SAMPLE_NOTIF_28 = `
GOVERNMENT OF INDIA
MINISTRY OF FINANCE
DEPARTMENT OF REVENUE
CENTRAL BOARD OF INDIRECT TAXES AND CUSTOMS
Exchange Rate Notification No.: 28/2026
Exchange Rate Notification for foreign currency rates for Imports & Exports w.e.f 02-10-2026
Currency Code Currency Name Currency Unit Rate Import Rate Export
AED UAE Dirham 1.0 26.95 25.4
AUD Australian Dollar 1.0 68.2 65.25
BHD Bahraini Dinar 1.0 270.15 239.65
CAD Canadian Dollar 1.0 68.55 66.4
CHF Swiss Franc 1.0 117.3 112.5
CNY Chinese Yuan 1.0 14.55 14.1
DKK Danish Kroner 1.0 14.7 14.35
EUR EURO 1.0 110.5 106.8
GBP Pound Sterling 1.0 129.25 125.2
HKD Hong Kong Dollar 1.0 12.4 12.1
JPY Japanese Yen 100.0 61.6 59.75
KRW Korean won 100.0 7.25 6.85
KWD Kuwaiti Dinar 1.0 320.9 301.8
NOK Norwegian Kroner 1.0 10.1 9.85
NZD New Zealand Dollar 1.0 55.3 52.55
QAR Qatari Riyal 1.0 27.15 25.6
SAR Saudi Arabian Riyal 1.0 26.8 24.25
SEK Swedish Kroner 1.0 9.7 9.45
SGD Singapore Dollar 1.0 76.35 73.8
TRY Turkish Lira 1.0 2 1.9
USD US Dollar 1.0 96.95 95.25
ZAR South African Rand 1.0 6 5.65
`;

const SAMPLE_NOTIF_29 = `
GOVERNMENT OF INDIA
MINISTRY OF FINANCE
DEPARTMENT OF REVENUE
CENTRAL BOARD OF INDIRECT TAXES AND CUSTOMS
Exchange Rate Notification No.: 29/2026
Exchange Rate Notification for foreign currency rates for Imports & Exports w.e.f 16-10-2026
Currency Code Currency Name Currency Unit Rate Import Rate Export
AED UAE Dirham 1.0 26.65 25.15
USD US Dollar 1.0 95.9 94.2
JPY Japanese Yen 100.0 61.2 59.35
KRW Korean won 100.0 7.25 6.85
`;

describe('CBIC Notification Parser', () => {
  it('correctly parses Notification 28/2026 with all 22 statutory currencies', () => {
    const result = parseCBICNotificationText(SAMPLE_NOTIF_28);

    expect(result.notificationNumber).toBe('Notification No. 28/2026');
    expect(result.effectiveFrom).toBe('2026-10-02');
    expect(result.rates).toHaveLength(22);

    // USD Check
    const usd = result.rates.find((r) => r.currencyCode === 'USD');
    expect(usd).toBeDefined();
    expect(usd?.rateImport).toBe(96.95);
    expect(usd?.rateExport).toBe(95.25);
    expect(usd?.currencyUnit).toBe(1.0);
    expect(usd?.effectiveRate).toBe(96.95);

    // JPY (Unit 100) Check
    const jpy = result.rates.find((r) => r.currencyCode === 'JPY');
    expect(jpy).toBeDefined();
    expect(jpy?.rateImport).toBe(61.6);
    expect(jpy?.currencyUnit).toBe(100.0);
    expect(jpy?.effectiveRate).toBeCloseTo(0.616, 4);

    // KRW (Unit 100) Check
    const krw = result.rates.find((r) => r.currencyCode === 'KRW');
    expect(krw).toBeDefined();
    expect(krw?.rateImport).toBe(7.25);
    expect(krw?.currencyUnit).toBe(100.0);
    expect(krw?.effectiveRate).toBeCloseTo(0.0725, 4);

    // Integer rate check (TRY)
    const tryRate = result.rates.find((r) => r.currencyCode === 'TRY');
    expect(tryRate).toBeDefined();
    expect(tryRate?.rateImport).toBe(2);
    expect(tryRate?.rateExport).toBe(1.9);
    expect(tryRate?.effectiveRate).toBe(2);
  });

  it('correctly parses Notification 29/2026 effective 2026-10-16', () => {
    const result = parseCBICNotificationText(SAMPLE_NOTIF_29);

    expect(result.notificationNumber).toBe('Notification No. 29/2026');
    expect(result.effectiveFrom).toBe('2026-10-16');
    expect(result.rates).toHaveLength(4);

    const usd = result.rates.find((r) => r.currencyCode === 'USD');
    expect(usd?.rateImport).toBe(95.9);

    const jpy = result.rates.find((r) => r.currencyCode === 'JPY');
    expect(jpy?.effectiveRate).toBeCloseTo(0.612, 4);
  });
});
