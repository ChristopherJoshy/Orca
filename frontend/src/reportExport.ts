import { customerFor, outstanding, type Invoice, type Money, type Payment, type WorkspaceData } from './model';

export type ReportExportKind = 'Revenue' | 'Invoicing' | 'Collections' | 'Receivables';

type ReportExportInput = {
  report: ReportExportKind;
  period: string;
  range: string;
  data: WorkspaceData;
  invoices: Invoice[];
  payments: Payment[];
  receivableInvoices: Invoice[];
  overdueInvoiceIds: string[];
  activeSubscriptionCount: number;
  monthlyRecurringRevenue: Money;
  invoicedTotal: Money;
  collectedTotal: Money;
};

function csvCell(value: unknown): string {
  const text = String(value ?? '');
  // Treat values with leading whitespace like formula prefixes as untrusted too.
  const safe = /^[\t\r]/.test(text) || /^[\u0000-\u0020]*[=+\-@]/.test(text) ? `'${text}` : text;
  return `"${safe.replaceAll('"', '""')}"`;
}

function encodeCsv(rows: unknown[][]): string {
  return `\uFEFF${rows.map((row) => row.map(csvCell).join(',')).join('\r\n')}\r\n`;
}

function csvRows(input: ReportExportInput): unknown[][] {
  const { report, period, range, data, invoices, payments, receivableInvoices, overdueInvoiceIds } = input;
  if (report === 'Revenue') {
    return [
      ['Metric', 'Amount', 'Currency', 'Selected period', 'Definition'],
      ['Monthly recurring revenue', input.monthlyRecurringRevenue, 'INR', range, 'Current run rate across active subscriptions'],
      ['Invoiced in period', input.invoicedTotal, 'INR', range, 'Confirmed and paid invoices by issue date'],
      ['Collected in period', input.collectedTotal, 'INR', range, 'Payment allocations by received date'],
      ['Active subscriptions', input.activeSubscriptionCount, '', range, 'Current active subscription count'],
      ['Report period', period, '', range, ''],
    ];
  }

  if (report === 'Invoicing') {
    return [
      ['Invoice', 'Customer', 'Status', 'Issued on', 'Due on', 'Billing period', 'Subtotal INR', 'Discount INR', 'Tax INR', 'Total INR', 'Paid INR', 'Outstanding INR', 'Selected period', 'Date range'],
      ...invoices.map((invoice) => [
        invoice.number, customerFor(data, invoice.customerId).company || customerFor(data, invoice.customerId).name,
        invoice.status, invoice.issueDate.slice(0, 10), invoice.dueDate.slice(0, 10), invoice.period,
        invoice.subtotal, invoice.discount, invoice.tax, invoice.total, invoice.paid, outstanding(invoice), period, range,
      ]),
    ];
  }

  if (report === 'Collections') {
    const invoiceById = new Map(data.invoices.map((invoice) => [invoice.id, invoice]));
    return [
      ['Payment', 'Customer', 'Invoice', 'Received on', 'Method', 'Note', 'Amount INR', 'Selected period', 'Date range'],
      ...payments.map((payment) => {
        const customer = customerFor(data, payment.customerId);
        return [payment.id, customer.company || customer.name, invoiceById.get(payment.invoiceId)?.number ?? payment.invoiceId, payment.date.slice(0, 10), payment.method, payment.note, payment.amount, period, range];
      }),
    ];
  }

  return [
    ['Invoice', 'Customer', 'Issued on', 'Due on', 'Status', 'Past due', 'Invoice total INR', 'Paid INR', 'Outstanding INR', 'Selected period', 'Date range'],
    ...receivableInvoices.map((invoice) => {
      const customer = customerFor(data, invoice.customerId);
      const balance = outstanding(invoice);
      return [invoice.number, customer.company || customer.name, invoice.issueDate.slice(0, 10), invoice.dueDate.slice(0, 10), invoice.status, overdueInvoiceIds.includes(invoice.id) ? 'Yes' : 'No', invoice.total, invoice.paid, balance, period, range];
    }),
  ];
}

export function downloadReportCsv(input: ReportExportInput): void {
  const body = encodeCsv(csvRows(input));
  const file = new Blob([body], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(file);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `orca-${input.report.toLowerCase()}-${new Date().toISOString().slice(0, 10)}.csv`;
  anchor.style.display = 'none';
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
