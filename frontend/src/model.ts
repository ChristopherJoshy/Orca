export type Money = string;
export type Cadence = 'Daily' | 'Weekly' | 'Monthly' | 'Yearly';
export type InvoiceStatus = 'Draft' | 'Confirmed' | 'Paid' | 'Cancelled';
export type SubscriptionStatus = 'Draft' | 'Quotation' | 'Confirmed' | 'Active' | 'Paused' | 'Closed';

export type Product = {
  id: string; name: string; type: string; price: Money; cost: Money; taxRateBps: number; notes: string;
  variants: { name: string; adjustment: Money }[]; active: boolean;
};
export type Plan = {
  id: string; name: string; price: Money; cadence: Cadence; taxRateBps: number; minimumQuantity: number;
  availableFrom: string; availableUntil: string; pauseAllowed: boolean; closeAllowed: boolean;
  renewAllowed: boolean; active: boolean;
};
export type Customer = {
  id: string; name: string; email: string; company: string; phone: string;
  status: 'Active' | 'Lead' | 'Past due'; since: string; initials: string;
};
export type Subscription = {
  id: string; number: string; customerId: string; planId: string; status: SubscriptionStatus;
  startedAt: string; renewsAt: string; amount: Money; seats: number; paymentTerms: string;
};
export type InvoiceLine = {
  description: string; quantity: number; unitPrice: Money; taxRateBps: number; discountBps: number;
  storedCalculation?: { subtotal: Money; discount: Money; taxable: Money; tax: Money; total: Money };
};
export type InvoiceSnapshotLine = {
  productId?: string; productVersion?: number; description: string; variant?: string; quantity: number;
  unitPrice: Money; rawSubtotal: Money; discountAmount: Money; taxableAmount: Money;
  taxRate: string; taxAmount: Money; lineTotal: Money; explanation: string[];
};
export type InvoiceSnapshot = {
  engineVersion: string; currency: string; businessId?: string; subscriptionId?: string; planId?: string;
  planVersion?: number; periodStart: string; periodEnd: string; discountId?: string; discountVersion?: number;
  subtotal: Money; discountTotal: Money; taxTotal: Money; total: Money;
  lines: InvoiceSnapshotLine[]; explanation: string[];
};
export type Invoice = {
  id: string; number: string; customerId: string; subscriptionId: string; issueDate: string;
  dueDate: string; status: InvoiceStatus; lines: InvoiceLine[]; subtotal: Money; discount: Money;
  tax: Money; total: Money; paid: Money; currency: 'INR'; period: string; calculationHash: string;
  snapshot?: InvoiceSnapshot;
};
export type Payment = { id: string; invoiceId: string; customerId: string; amount: Money; date: string; method: string; note: string };
export type Quotation = { id: string; number: string; customerId: string; title: string; date: string; validUntil: string; amount: Money; status: string };
export type QuoteTemplate = { id: string; name: string; validDays: number; planId: string; lines: { productId: string; name: string; quantity: number; unitPrice: Money; taxRate: string }[] };
export type Discount = { id: string; name: string; kind: 'Percentage' | 'Fixed'; value: string; scope: string; minimum: Money; minimumQuantity: number; usage: number; limit: number; startsAt: string; endsAt: string; active: boolean };
export type TaxRule = { id: string; name: string; region: string; rateBps: number; active: boolean };
export type ExportJob = { id: string; name: string; createdAt: string; status: string; counts?: Record<string, number> };
export type Staff = { id: string; name: string; email: string; role: 'Admin' | 'Staff'; status: string; initials: string };
export type Approval = { id: string; title: string; detail: string; amount?: Money; target: string; createdAt: string; status: 'Needs review' | 'Approved' | 'Declined'; action: string; executionResult?: Record<string, unknown> };

export type WorkspaceData = {
  products: Product[]; plans: Plan[]; customers: Customer[]; subscriptions: Subscription[];
  invoices: Invoice[]; payments: Payment[]; quotations: Quotation[]; discounts: Discount[];
  quoteTemplates: QuoteTemplate[]; taxes: TaxRule[]; exports: ExportJob[]; staff: Staff[]; approvals: Approval[];
};

export const today = new Date().toISOString().slice(0, 10);
export function emptyWorkspaceData(): WorkspaceData {
  return {
    products: [], plans: [], customers: [], subscriptions: [], invoices: [], payments: [],
    quotations: [], quoteTemplates: [], discounts: [], taxes: [], exports: [], staff: [], approvals: [],
  };
}

// Money stays a decimal string over APIs and in persisted UI state. Convert to integer
// paise only for arithmetic; Number is used solely by formatters and chart geometry.
export function toPaise(value: Money): bigint {
  if (!/^-?\d+(?:\.\d{1,2})?$/.test(value)) throw new Error('Enter an amount with up to two decimal places.');
  const negative = value.startsWith('-');
  const unsigned = negative ? value.slice(1) : value;
  const [rupees, fraction = ''] = unsigned.split('.');
  const result = BigInt(rupees) * 100n + BigInt(fraction.padEnd(2, '0'));
  return negative ? -result : result;
}

export function fromPaise(value: bigint): Money {
  const sign = value < 0n ? '-' : '';
  const absolute = value < 0n ? -value : value;
  return `${sign}${absolute / 100n}.${(absolute % 100n).toString().padStart(2, '0')}`;
}

export function addMoney(...values: Money[]): Money {
  return fromPaise(values.reduce((sum, value) => sum + toPaise(value), 0n));
}

export function subtractMoney(left: Money, right: Money): Money {
  return fromPaise(toPaise(left) - toPaise(right));
}

export function multiplyMoney(value: Money, quantity: number): Money {
  if (!Number.isSafeInteger(quantity)) throw new Error('Quantity must be a whole number.');
  return fromPaise(toPaise(value) * BigInt(quantity));
}

export function ratioMoney(value: Money, numerator: number, denominator: number): Money {
  if (!Number.isSafeInteger(numerator) || !Number.isSafeInteger(denominator) || numerator < 0 || denominator <= 0) throw new Error('Ratio must use non-negative whole numbers and a positive denominator.');
  const scaled = toPaise(value) * BigInt(numerator);
  const divisor = BigInt(denominator);
  const absolute = scaled < 0n ? -scaled : scaled;
  const rounded = (absolute + divisor / 2n) / divisor;
  return fromPaise(scaled < 0n ? -rounded : rounded);
}

export function percentageMoney(value: Money, basisPoints: number): Money {
  if (!Number.isSafeInteger(basisPoints) || basisPoints < 0) throw new Error('Percentage must be a non-negative whole basis-point value.');
  const numerator = toPaise(value) * BigInt(basisPoints);
  const divisor = 10000n;
  const absolute = numerator < 0n ? -numerator : numerator;
  const rounded = (absolute + divisor / 2n) / divisor;
  return fromPaise(numerator < 0n ? -rounded : rounded);
}

export function parseRateBasisPoints(value: string): number {
  const text = value.trim().replace(/%$/, '');
  if (!/^\d+(?:\.\d{1,2})?$/.test(text)) throw new Error('Enter a percentage with up to two decimal places.');
  const [whole, fraction = ''] = text.split('.');
  const basisPoints = BigInt(whole) * 100n + BigInt(fraction.padEnd(2, '0'));
  if (basisPoints > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error('Percentage is too large.');
  return Number(basisPoints);
}

export function money(value: Money): string {
  const amount = Number(value);
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 2, minimumFractionDigits: 2 }).format(Number.isFinite(amount) ? amount : 0);
}

export function shortMoney(value: Money): string {
  const amount = Number(value);
  const abs = Math.abs(amount);
  if (abs >= 10000000) return `₹${(amount / 10000000).toFixed(1)}Cr`;
  if (abs >= 100000) return `₹${(amount / 100000).toFixed(1)}L`;
  if (abs >= 1000) return `₹${(amount / 1000).toFixed(1)}K`;
  return money(value);
}

export function customerFor(data: WorkspaceData, id: string): Customer {
  return data.customers.find((customer) => customer.id === id) ?? data.customers[0] ?? { id: '', name: 'Unknown customer', email: '', company: '', phone: '', status: 'Lead', since: '', initials: '?' };
}

export function outstanding(invoice: Invoice): Money {
  return toPaise(invoice.total) > toPaise(invoice.paid) ? subtractMoney(invoice.total, invoice.paid) : '0.00';
}

export function isOverdue(invoice: Invoice): boolean {
  return invoice.status === 'Confirmed' && toPaise(outstanding(invoice)) > 0n && invoice.dueDate < today;
}

export function calculateLines(lines: InvoiceLine[]) {
  let subtotal: Money = '0.00';
  let discount: Money = '0.00';
  let tax: Money = '0.00';
  const steps = lines.map((line) => {
    const gross = line.storedCalculation?.subtotal ?? multiplyMoney(line.unitPrice, line.quantity);
    const lineDiscount = line.storedCalculation?.discount ?? percentageMoney(gross, line.discountBps);
    const taxable = line.storedCalculation?.taxable ?? subtractMoney(gross, lineDiscount);
    const lineTax = line.storedCalculation?.tax ?? percentageMoney(taxable, line.taxRateBps);
    const total = line.storedCalculation?.total ?? addMoney(taxable, lineTax);
    subtotal = addMoney(subtotal, gross);
    discount = addMoney(discount, lineDiscount);
    tax = addMoney(tax, lineTax);
    return { ...line, gross, lineDiscount, taxable, lineTax, total };
  });
  return { steps, subtotal, discount, tax, total: addMoney(subtractMoney(subtotal, discount), tax) };
}
