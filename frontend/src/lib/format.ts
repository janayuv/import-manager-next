export function formatINR(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '₹0.00';
  return '₹' + val.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function formatCurr(val?: number, curr = 'USD'): string {
  if (val === undefined || val === null || isNaN(val)) return `0.00 ${curr}`;
  return `${val.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${curr}`;
}
