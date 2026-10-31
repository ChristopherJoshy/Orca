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
export type Discount = { id: string; name: string; kind: 'Percentage' | 'Fixed'; value: string; scope: string; minimum: Money; minimumQuantity: number; usage: number; limit: number; endsAt: string; active: boolean };
export type TaxRule = { id: string; name: string; region: string; rateBps: number; active: boolean };
export type ExportJob = { id: string; name: string; createdAt: string; status: string; counts?: Record<string, number> };
export type Staff = { id: string; name: string; email: string; role: 'Admin' | 'Staff'; status: string; initials: string };
export type Approval = { id: string; title: string; detail: string; amount?: Money; target: string; createdAt: string; status: 'Needs review' | 'Approved' | 'Declined'; action: string; executionResult?: Record<string, unknown> };

export type WorkspaceData = {
  products: Product[]; plans: Plan[]; customers: Customer[]; subscriptions: Subscription[];
  invoices: Invoice[]; payments: Payment[]; quotations: Quotation[]; discounts: Discount[];
  taxes: TaxRule[]; exports: ExportJob[]; staff: Staff[]; approvals: Approval[];
};

export const today = new Date().toISOString().slice(0, 10);
const daysFromNow = (days: number) => new Date(Date.now() + days * 86400000).toISOString().slice(0, 10);

export const demoData: WorkspaceData = {
  products: [
    { id: 'p1', name: 'Orca Business', type: 'Software', price: '2499.00', cost: '620.00', taxRateBps: 1800, notes: 'Core workspace subscription', variants: [{ name: 'Standard', adjustment: '0.00' }, { name: 'Enterprise', adjustment: '560.00' }], active: true },
    { id: 'p2', name: 'Workflow automation', type: 'Add-on', price: '899.00', cost: '180.00', taxRateBps: 1800, notes: 'Additional automation pack', variants: [], active: true },
    { id: 'p3', name: 'Priority support', type: 'Service', price: '1200.00', cost: '400.00', taxRateBps: 1800, notes: 'Business-hours priority support', variants: [], active: true },
  ],
  plans: [
    { id: 'pl1', name: 'Business monthly', price: '2499.00', cadence: 'Monthly', taxRateBps: 1800, minimumQuantity: 1, availableFrom: '2025-01-01', availableUntil: '', pauseAllowed: true, closeAllowed: true, renewAllowed: true, active: true },
    { id: 'pl2', name: 'Business annual', price: '24990.00', cadence: 'Yearly', taxRateBps: 1800, minimumQuantity: 5, availableFrom: '2025-01-01', availableUntil: '', pauseAllowed: false, closeAllowed: true, renewAllowed: true, active: true },
    { id: 'pl3', name: 'Starter weekly', price: '699.00', cadence: 'Weekly', taxRateBps: 1800, minimumQuantity: 1, availableFrom: '2025-06-01', availableUntil: '', pauseAllowed: true, closeAllowed: true, renewAllowed: false, active: true },
  ],
  customers: [
    { id: 'c1', name: 'Maya Chen', email: 'maya@northstar.example', company: 'Northstar Labs', phone: '+91 98765 43210', status: 'Active', since: '2024-10-12', initials: 'MC' },
    { id: 'c2', name: 'Arjun Mehta', email: 'arjun@fieldnote.example', company: 'Fieldnote Studio', phone: '+91 98111 20984', status: 'Past due', since: '2024-08-06', initials: 'AM' },
    { id: 'c3', name: 'Priya Nair', email: 'priya@lumen.example', company: 'Lumen Design Co.', phone: '+91 98990 33418', status: 'Active', since: '2025-01-23', initials: 'PN' },
    { id: 'c4', name: 'Daniel Kim', email: 'daniel@atlasworks.example', company: 'Atlas Works', phone: '+91 90000 12453', status: 'Active', since: '2024-06-03', initials: 'DK' },
    { id: 'c5', name: 'Aanya Rao', email: 'aanya@monocle.example', company: 'Monocle Commerce', phone: '+91 94452 61020', status: 'Lead', since: '2025-03-17', initials: 'AR' },
  ],
  subscriptions: [
    { id: 's1', number: 'SUB-1048', customerId: 'c1', planId: 'pl2', status: 'Active', startedAt: '2025-10-31', renewsAt: today, amount: '124950.00', seats: 5, paymentTerms: 'Net 30' },
    { id: 's2', number: 'SUB-1047', customerId: 'c2', planId: 'pl1', status: 'Active', startedAt: daysFromNow(-80), renewsAt: daysFromNow(9), amount: '7497.00', seats: 3, paymentTerms: 'Due on receipt' },
    { id: 's3', number: 'SUB-1046', customerId: 'c3', planId: 'pl1', status: 'Active', startedAt: daysFromNow(-40), renewsAt: daysFromNow(3), amount: '4998.00', seats: 2, paymentTerms: 'Net 15' },
    { id: 's4', number: 'SUB-1045', customerId: 'c4', planId: 'pl2', status: 'Active', startedAt: daysFromNow(-357), renewsAt: daysFromNow(8), amount: '124950.00', seats: 5, paymentTerms: 'Net 30' },
    { id: 's5', number: 'SUB-1044', customerId: 'c5', planId: 'pl1', status: 'Quotation', startedAt: daysFromNow(2), renewsAt: daysFromNow(32), amount: '2499.00', seats: 1, paymentTerms: 'Net 15' },
    { id: 's6', number: 'SUB-1043', customerId: 'c1', planId: 'pl1', status: 'Paused', startedAt: daysFromNow(-150), renewsAt: daysFromNow(20), amount: '2499.00', seats: 1, paymentTerms: 'Net 30' },
  ],
  invoices: [
    { id: 'i1', number: 'INV-2026-0108', customerId: 'c1', subscriptionId: 's1', issueDate: daysFromNow(-3), dueDate: daysFromNow(27), status: 'Paid', lines: [{ description: 'Business annual · 5 seats', quantity: 5, unitPrice: '24990.00', taxRateBps: 1800, discountBps: 0 }], subtotal: '124950.00', discount: '0.00', tax: '22491.00', total: '147441.00', paid: '147441.00', currency: 'INR', period: 'Oct 31, 2025 – Oct 30, 2026', calculationHash: '' },
    { id: 'i2', number: 'INV-2026-0107', customerId: 'c2', subscriptionId: 's2', issueDate: daysFromNow(-24), dueDate: daysFromNow(-9), status: 'Confirmed', lines: [{ description: 'Business monthly · Standard', quantity: 3, unitPrice: '2499.00', taxRateBps: 1800, discountBps: 1000 }], subtotal: '7497.00', discount: '749.70', tax: '1214.51', total: '7961.81', paid: '4000.00', currency: 'INR', period: 'Sep 6 – Oct 5, 2026', calculationHash: '' },
    { id: 'i3', number: 'INV-2026-0106', customerId: 'c3', subscriptionId: 's3', issueDate: daysFromNow(-6), dueDate: daysFromNow(9), status: 'Confirmed', lines: [{ description: 'Business monthly · Standard', quantity: 2, unitPrice: '2499.00', taxRateBps: 1800, discountBps: 0 }], subtotal: '4998.00', discount: '0.00', tax: '899.64', total: '5897.64', paid: '0.00', currency: 'INR', period: 'Oct 23 – Nov 22, 2026', calculationHash: '' },
    { id: 'i4', number: 'INV-2026-0105', customerId: 'c4', subscriptionId: 's4', issueDate: daysFromNow(-18), dueDate: daysFromNow(-3), status: 'Paid', lines: [{ description: 'Business annual · 5 seats', quantity: 5, unitPrice: '24990.00', taxRateBps: 1800, discountBps: 0 }], subtotal: '124950.00', discount: '0.00', tax: '22491.00', total: '147441.00', paid: '147441.00', currency: 'INR', period: 'Nov 8, 2025 – Nov 7, 2026', calculationHash: '' },
    { id: 'i5', number: 'INV-2026-0104', customerId: 'c1', subscriptionId: 's1', issueDate: daysFromNow(-35), dueDate: daysFromNow(-5), status: 'Draft', lines: [{ description: 'Business annual · 5 seats', quantity: 5, unitPrice: '24990.00', taxRateBps: 1800, discountBps: 0 }], subtotal: '124950.00', discount: '0.00', tax: '22491.00', total: '147441.00', paid: '0.00', currency: 'INR', period: 'Oct 31, 2024 – Oct 30, 2025', calculationHash: '' },
  ],
  payments: [
    { id: 'pay1', invoiceId: 'i1', customerId: 'c1', amount: '147441.00', date: daysFromNow(-3), method: 'Bank transfer', note: 'October renewal' },
    { id: 'pay2', invoiceId: 'i2', customerId: 'c2', amount: '4000.00', date: daysFromNow(-20), method: 'UPI', note: 'Part payment' },
    { id: 'pay3', invoiceId: 'i4', customerId: 'c4', amount: '147441.00', date: daysFromNow(-16), method: 'Card', note: 'Invoice paid in full' },
  ],
  quotations: [
    { id: 'q1', number: 'QUO-2026-014', customerId: 'c5', title: 'Business plan · 1 seat', date: daysFromNow(-1), validUntil: daysFromNow(13), amount: '2499.00', status: 'Sent' },
    { id: 'q2', number: 'QUO-2026-013', customerId: 'c3', title: 'Annual business · 5 seats', date: daysFromNow(-6), validUntil: daysFromNow(8), amount: '124950.00', status: 'Accepted' },
    { id: 'q3', number: 'QUO-2026-012', customerId: 'c2', title: 'Business monthly · 3 seats', date: daysFromNow(-12), validUntil: daysFromNow(2), amount: '7497.00', status: 'Draft' },
  ],
  discounts: [
    { id: 'd1', name: 'Annual commitment', kind: 'Percentage', value: '10', scope: 'Subscriptions', minimum: '24990.00', minimumQuantity: 1, usage: 17, limit: 40, endsAt: daysFromNow(45), active: true },
    { id: 'd2', name: 'Launch credit', kind: 'Fixed', value: '1500.00', scope: 'Orca Business', minimum: '5000.00', minimumQuantity: 1, usage: 8, limit: 25, endsAt: daysFromNow(18), active: true },
  ],
  taxes: [
    { id: 't1', name: 'GST', region: 'India · Standard rated', rateBps: 1800, active: true },
    { id: 't2', name: 'GST', region: 'India · Zero rated', rateBps: 0, active: true },
  ],
  exports: [],
  staff: [
    { id: 'u1', name: 'Morgan Ellis', email: 'morgan@brightloop.example', role: 'Admin', status: 'Active', initials: 'ME' },
    { id: 'u2', name: 'Amara Patel', email: 'amara@brightloop.example', role: 'Staff', status: 'Active', initials: 'AP' },
    { id: 'u3', name: 'Ethan Wilson', email: 'ethan@brightloop.example', role: 'Staff', status: 'Invited', initials: 'EW' },
  ],
  approvals: [
    { id: 'a1', title: 'Send overdue reminder', detail: 'Email Arjun Mehta at Fieldnote Studio about invoice INV-2026-0107. The outstanding balance is ₹3,961.81.', amount: '3961.81', target: 'INV-2026-0107', createdAt: '2 min ago', status: 'Needs review', action: 'Send collection email' },
    { id: 'a2', title: 'Record payment allocation', detail: 'Apply a ₹2,000.00 UPI payment to invoice INV-2026-0107. New outstanding balance: ₹1,961.81.', amount: '2000.00', target: 'INV-2026-0107', createdAt: '18 min ago', status: 'Needs review', action: 'Allocate payment' },
  ],
};

export function cloneDemo(): WorkspaceData {
  const data = JSON.parse(JSON.stringify(demoData)) as WorkspaceData;
  const annualLine = (quantity: number): InvoiceSnapshotLine => ({
    description: `Business annual · ${quantity} seats`, quantity, unitPrice: '24990.00', rawSubtotal: '124950.00',
    discountAmount: '0.00', taxableAmount: '124950.00', taxRate: '18', taxAmount: '22491.00', lineTotal: '147441.00',
    explanation: ['5 × ₹24,990.00 = ₹124,950.00', 'Discount ₹0.00; taxable amount ₹124,950.00', 'Tax 18% on ₹124,950.00 = ₹22,491.00', 'Line total ₹147,441.00'],
  });
  const snapshots: Record<string, InvoiceSnapshot> = {
    i1: { engineVersion: 'demo-fixture-1', currency: 'INR', periodStart: '2025-10-31', periodEnd: '2026-10-31', subtotal: '124950.00', discountTotal: '0.00', taxTotal: '22491.00', total: '147441.00', lines: [annualLine(5)], explanation: ['Demo fixture evidence; no canonical hash is supplied.', 'Tax is applied after discount.', 'Line amount is rounded half-up to INR paise.'] },
    i2: { engineVersion: 'demo-fixture-1', currency: 'INR', periodStart: '2026-09-06', periodEnd: '2026-10-06', subtotal: '7497.00', discountTotal: '749.70', taxTotal: '1214.51', total: '7961.81', lines: [{ description: 'Business monthly · Standard', quantity: 3, unitPrice: '2499.00', rawSubtotal: '7497.00', discountAmount: '749.70', taxableAmount: '6747.30', taxRate: '18', taxAmount: '1214.51', lineTotal: '7961.81', explanation: ['3 × ₹2,499.00 = ₹7,497.00', 'Discount ₹749.70; taxable amount ₹6,747.30', 'Tax 18% on ₹6,747.30 = ₹1,214.51', 'Line total ₹7,961.81'] }], explanation: ['Demo fixture evidence; no canonical hash is supplied.', 'Tax is applied after discount.', 'Line amount is rounded half-up to INR paise.'] },
    i3: { engineVersion: 'demo-fixture-1', currency: 'INR', periodStart: '2026-10-23', periodEnd: '2026-11-23', subtotal: '4998.00', discountTotal: '0.00', taxTotal: '899.64', total: '5897.64', lines: [{ description: 'Business monthly · Standard', quantity: 2, unitPrice: '2499.00', rawSubtotal: '4998.00', discountAmount: '0.00', taxableAmount: '4998.00', taxRate: '18', taxAmount: '899.64', lineTotal: '5897.64', explanation: ['2 × ₹2,499.00 = ₹4,998.00', 'Discount ₹0.00; taxable amount ₹4,998.00', 'Tax 18% on ₹4,998.00 = ₹899.64', 'Line total ₹5,897.64'] }], explanation: ['Demo fixture evidence; no canonical hash is supplied.', 'Tax is applied after discount.', 'Line amount is rounded half-up to INR paise.'] },
    i4: { engineVersion: 'demo-fixture-1', currency: 'INR', periodStart: '2025-11-08', periodEnd: '2026-11-08', subtotal: '124950.00', discountTotal: '0.00', taxTotal: '22491.00', total: '147441.00', lines: [annualLine(5)], explanation: ['Demo fixture evidence; no canonical hash is supplied.', 'Tax is applied after discount.', 'Line amount is rounded half-up to INR paise.'] },
    i5: { engineVersion: 'demo-fixture-1', currency: 'INR', periodStart: '2024-10-31', periodEnd: '2025-10-31', subtotal: '124950.00', discountTotal: '0.00', taxTotal: '22491.00', total: '147441.00', lines: [annualLine(5)], explanation: ['Demo fixture evidence; no canonical hash is supplied.', 'Tax is applied after discount.', 'Line amount is rounded half-up to INR paise.'] },
  };
  for (const invoice of data.invoices) {
    const snapshot = snapshots[invoice.id];
    if (snapshot) {
      invoice.snapshot = snapshot;
      invoice.lines = snapshot.lines.map((line) => ({
        description: line.description, quantity: line.quantity, unitPrice: line.unitPrice,
        taxRateBps: parseRateBasisPoints(line.taxRate),
        discountBps: invoice.id === 'i2' ? 1000 : 0,
        storedCalculation: { subtotal: line.rawSubtotal, discount: line.discountAmount, taxable: line.taxableAmount, tax: line.taxAmount, total: line.lineTotal },
      }));
    }
    invoice.calculationHash = '';
  }
  return data;
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
