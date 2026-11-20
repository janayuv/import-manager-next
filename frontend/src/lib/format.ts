export function formatINR(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '₹0.00';
  return '₹' + val.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function formatCurr(val?: number, curr = 'USD'): string {
  if (val === undefined || val === null || isNaN(val)) return `0.00 ${curr}`;
  return `${val.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${curr}`;
}

export function formatTransitDateDisplay(dStr?: string): string {
  if (!dStr) return '';
  const d = new Date(dStr);
  if (isNaN(d.getTime())) return dStr;
  const day = String(d.getDate()).padStart(2, '0');
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  const mon = months[d.getMonth()];
  const yr = d.getFullYear();
  return `${day}-${mon}-${yr}`;
}
