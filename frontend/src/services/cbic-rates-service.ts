import * as CBICBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/cbic_rates/service.js';
import type {
  CBICExchangeRate,
  CBICExchangeRateInput,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  CBICExchangeRate,
  CBICExchangeRateInput,
};

export const cbicRatesService = {
  list(currency = ''): Promise<CBICExchangeRate[]> {
    return CBICBindings.List(currency).then((rows) => rows ?? []);
  },
  getEffectiveRate(currency: string, targetDate = ''): Promise<CBICExchangeRate> {
    return CBICBindings.GetEffectiveRate(currency, targetDate);
  },
  create(input: CBICExchangeRateInput): Promise<CBICExchangeRate> {
    return CBICBindings.Create(input);
  },
  async createBatch(items: CBICExchangeRateInput[]): Promise<{ created: number; errors: string[] }> {
    let created = 0;
    const errors: string[] = [];
    for (const item of items) {
      try {
        await CBICBindings.Create(item);
        created++;
      } catch (err) {
        errors.push(`${item.currencyCode}: ${err instanceof Error ? err.message : String(err)}`);
      }
    }
    return { created, errors };
  },
  delete(id: number): Promise<void> {
    return CBICBindings.Delete(id);
  },
};
