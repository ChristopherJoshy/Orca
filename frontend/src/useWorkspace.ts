import { useCallback, useEffect, useState } from 'react';
import { apiAvailable, apiRequest, ApiError, post, readItemsResult } from './api';
import { addMoney, cloneDemo, fromPaise, multiplyMoney, parseRateBasisPoints, subtractMoney, toPaise, type InvoiceSnapshot, type InvoiceSnapshotLine, type WorkspaceData } from './model';

export type ApiState = 'checking' | 'online' | 'offline';
export type DataMode = 'checking' | 'demo' | 'workspace' | 'auth-required';

function initialData(): WorkspaceData {
  try {
    const saved = localStorage.getItem('orca-demo-v2');
    if (saved) return { ...cloneDemo(), ...JSON.parse(saved) as Partial<WorkspaceData> };
  } catch { /* Local demo state is recoverable from the built-in examples. */ }
  return cloneDemo();
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {};
}

function asMoney(value: unknown): string {
  if (typeof value === 'string' && /^-?\d+\.\d{2}$/.test(value)) return value;
  throw new Error('The API returned money outside the decimal-string contract.');
}

function safeMap<T, U>(values: T[] | null, map: (value: T) => U): U[] | null {
  if (!values) return null;
  try { return values.map(map); } catch { return null; }
}

function productFromApi(value: unknown) {
  const p = asRecord(value);
  return {
    id: String(p.id ?? crypto.randomUUID()), name: String(p.name ?? 'Product'), type: String(p.type ?? 'Software'),
    price: asMoney(p.salesPrice ?? p.price), cost: asMoney(p.costPrice ?? p.cost), taxRateBps: typeof p.taxRateBps === 'number' ? p.taxRateBps : rateBasisPoints(p.taxRate), notes: String(p.notes ?? ''),
    variants: Array.isArray(p.variants) ? p.variants.map((item) => {
      const variant = asRecord(item);
      const variantName = [variant.name, variant.value].filter(Boolean).map(String).join(' · ');
      return { name: variantName || 'Variant', adjustment: asMoney(variant.adjustment ?? variant.priceAdjustment) };
    }) : [], active: p.active !== false,
  };
}

function customerFromApi(value: unknown) {
  const c = asRecord(value);
  const name = String(c.name ?? c.fullName ?? 'Customer');
  return {
    id: String(c.id ?? crypto.randomUUID()), name, email: String(c.email ?? ''),
    company: String(c.company ?? c.companyName ?? ''), phone: String(c.phone ?? ''),
    status: (String(c.status ?? 'Active') === 'Past due' ? 'Past due' : String(c.status ?? 'Active') === 'Lead' ? 'Lead' : 'Active') as 'Active' | 'Lead' | 'Past due',
    since: String(c.since ?? c.createdAt ?? new Date().toISOString().slice(0, 10)), initials: name.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase(),
  };
}

function planFromApi(value: unknown) {
  const p = asRecord(value);
  const rawCadence = String(p.cadence ?? p.interval ?? p.frequency ?? 'Monthly');
  const cadence = rawCadence.charAt(0).toUpperCase() + rawCadence.slice(1).toLowerCase();
  return {
    id: String(p.id ?? crypto.randomUUID()), name: String(p.name ?? 'Plan'), price: asMoney(p.salesPrice ?? p.price ?? p.amount),
    cadence: (['Daily', 'Weekly', 'Monthly', 'Yearly'].includes(cadence) ? cadence : 'Monthly') as 'Daily' | 'Weekly' | 'Monthly' | 'Yearly',
    taxRateBps: typeof p.taxRateBps === 'number' ? p.taxRateBps : rateBasisPoints(p.taxRate),
    minimumQuantity: Number(p.minimumQuantity ?? p.minQuantity ?? 1), availableFrom: String(p.availableFrom ?? ''), availableUntil: String(p.availableUntil ?? ''),
    pauseAllowed: p.pauseAllowed !== false && p.allowPause !== false,
    closeAllowed: p.closeAllowed !== false && p.allowEarlyClose !== false,
    renewAllowed: p.renewAllowed !== false && p.allowRenew !== false, active: p.active !== false,
  };
}

function subscriptionFromApi(value: unknown) {
  const s = asRecord(value);
  const lines = Array.isArray(s.lines) ? s.lines.map(asRecord) : [];
  const seats = lines.reduce((sum, line) => sum + Math.max(0, Number(line.quantity ?? 0)), 0) || Number(s.quantity ?? 1);
  const amount = lines.reduce((sum, line) => addMoney(sum, multiplyMoney(asMoney(line.unitPrice ?? '0.00'), Math.max(0, Number(line.quantity ?? 0)))), '0.00');
  const rawStatus = String(s.status ?? 'Draft');
  const status = rawStatus.charAt(0).toUpperCase() + rawStatus.slice(1).toLowerCase();
  return {
    id: String(s.id ?? crypto.randomUUID()), number: String(s.number ?? s.subscriptionNumber ?? 'SUB-0000'),
    customerId: String(s.customerId ?? ''), planId: String(s.planId ?? ''), status: status as WorkspaceData['subscriptions'][number]['status'],
    startedAt: String(s.startedAt ?? s.startDate ?? ''), renewsAt: String(s.renewsAt ?? s.expiryDate ?? ''),
    amount: asMoney(s.amount ?? s.total ?? (amount === '0.00' ? '0.00' : amount)), seats,
    paymentTerms: Number(s.paymentTermsDays) > 0 ? `Net ${Number(s.paymentTermsDays)}` : String(s.paymentTerms ?? 'Due on receipt'),
  };
}

function rateBasisPoints(value: unknown): number {
  const text = String(value ?? '0').trim();
  return parseRateBasisPoints(text || '0');
}

function percentBasisPoints(discount: string, subtotal: string): number {
  const base = toPaise(subtotal);
  if (base <= 0n) return 0;
  return Number((toPaise(discount) * 10000n + base / 2n) / base);
}

function invoiceFromApi(value: unknown) {
  const i = asRecord(value);
  const rawSnapshot = i.snapshot ?? i.calculationSnapshot;
  const snapshotRecord = rawSnapshot && typeof rawSnapshot === 'object' ? asRecord(rawSnapshot) : null;
  const snapshot: InvoiceSnapshot | undefined = snapshotRecord ? {
    engineVersion: String(snapshotRecord.engineVersion ?? ''),
    currency: String(snapshotRecord.currency ?? i.currency ?? 'INR'),
    businessId: String(snapshotRecord.businessId ?? ''),
    subscriptionId: String(snapshotRecord.subscriptionId ?? i.subscriptionId ?? ''),
    planId: String(snapshotRecord.planId ?? ''),
    planVersion: Number(snapshotRecord.planVersion ?? 0),
    periodStart: String(snapshotRecord.periodStart ?? i.periodStart ?? ''),
    periodEnd: String(snapshotRecord.periodEnd ?? i.periodEnd ?? ''),
    discountId: String(snapshotRecord.discountId ?? ''),
    discountVersion: Number(snapshotRecord.discountVersion ?? 0),
    subtotal: asMoney(snapshotRecord.subtotal ?? i.subtotal ?? '0.00'),
    discountTotal: asMoney(snapshotRecord.discountTotal ?? i.discountTotal ?? '0.00'),
    taxTotal: asMoney(snapshotRecord.taxTotal ?? i.taxTotal ?? '0.00'),
    total: asMoney(snapshotRecord.total ?? i.total ?? '0.00'),
    lines: (Array.isArray(snapshotRecord.lines) ? snapshotRecord.lines : []).map((rawLine): InvoiceSnapshotLine => {
      const line = asRecord(rawLine);
      return {
        productId: String(line.productId ?? ''), productVersion: Number(line.productVersion ?? 0),
        description: String(line.description ?? line.name ?? 'Invoice line'), variant: String(line.variant ?? ''),
        quantity: Math.max(0, Number(line.quantity ?? 0)), unitPrice: asMoney(line.unitPrice ?? '0.00'),
        rawSubtotal: asMoney(line.rawSubtotal ?? line.subtotal ?? '0.00'),
        discountAmount: asMoney(line.discountAmount ?? line.discount ?? '0.00'),
        taxableAmount: asMoney(line.taxableAmount ?? '0.00'), taxRate: String(line.taxRate ?? '0'),
        taxAmount: asMoney(line.taxAmount ?? '0.00'), lineTotal: asMoney(line.lineTotal ?? '0.00'),
        explanation: Array.isArray(line.explanation) ? line.explanation.map(String) : [],
      };
    }),
    explanation: Array.isArray(snapshotRecord.explanation) ? snapshotRecord.explanation.map(String) : [],
  } : undefined;
  const total = asMoney(i.total);
  const outstanding = asMoney(i.outstanding ?? i.balance ?? total);
  const paidMoney = fromPaise(toPaise(total) - toPaise(outstanding));
  const status = String(i.status ?? 'Draft');
  const lineRecords = snapshot?.lines ?? (Array.isArray(i.lines) ? i.lines : []);
  const lines = lineRecords.map((rawLine) => {
    const line = asRecord(rawLine);
    const rawSubtotal = asMoney(line.rawSubtotal ?? line.subtotal ?? '0.00');
    const discountAmount = asMoney(line.discountAmount ?? line.discount ?? '0.00');
    return {
      description: String(line.description ?? line.name ?? 'Invoice line'),
      quantity: Math.max(0, Number(line.quantity ?? 0)),
      unitPrice: asMoney(line.unitPrice ?? '0.00'),
      taxRateBps: typeof line.taxRateBps === 'number' ? line.taxRateBps : rateBasisPoints(line.taxRate),
      discountBps: percentBasisPoints(discountAmount, rawSubtotal),
      ...(snapshot ? { storedCalculation: {
        subtotal: rawSubtotal,
        discount: discountAmount,
        taxable: asMoney(line.taxableAmount ?? subtractMoney(rawSubtotal, discountAmount)),
        tax: asMoney(line.taxAmount ?? '0.00'),
        total: asMoney(line.lineTotal ?? '0.00'),
      } } : {}),
    };
  });
  const periodStart = String(snapshot?.periodStart ?? i.periodStart ?? '');
  const periodEnd = String(snapshot?.periodEnd ?? i.periodEnd ?? '');
  const period = periodStart && periodEnd ? `${periodStart.slice(0, 10)} – ${periodEnd.slice(0, 10)}` : '';
  return {
    id: String(i.id ?? crypto.randomUUID()), number: String(i.number ?? 'INV-0000'), customerId: String(i.customerId ?? ''), subscriptionId: String(i.subscriptionId ?? ''),
    issueDate: String(i.issueDate ?? i.createdAt ?? ''), dueDate: String(i.dueDate ?? i.dueAt ?? ''), status: status as WorkspaceData['invoices'][number]['status'],
    lines,
    subtotal: asMoney(i.subtotal ?? snapshot?.subtotal ?? total),
    discount: asMoney(i.discount ?? i.discountTotal ?? snapshot?.discountTotal ?? '0.00'),
    tax: asMoney(i.tax ?? i.taxTotal ?? snapshot?.taxTotal ?? '0.00'), total, paid: paidMoney,
    currency: 'INR' as const, period: String(i.period ?? period), calculationHash: String(i.calculationHash ?? ''), snapshot,
  };
}

function quotationFromApi(value: unknown) {
  const q = asRecord(value);
  return {
    id: String(q.id ?? crypto.randomUUID()), number: String(q.number ?? 'QUO-0000'),
    customerId: String(q.customerId ?? ''), title: String(q.title ?? q.name ?? 'Quotation'),
    date: String(q.date ?? q.createdAt ?? ''), validUntil: String(q.validUntil ?? q.expiresAt ?? ''),
    amount: asMoney(q.amount ?? q.total ?? '0.00'), status: String(q.status ?? 'Draft'),
  };
}

function paymentFromApi(value: unknown) {
  const p = asRecord(value);
  return {
    id: String(p.id ?? crypto.randomUUID()), invoiceId: String(p.invoiceId ?? ''), customerId: String(p.customerId ?? ''),
    amount: asMoney(p.amount), date: String(p.date ?? p.receivedAt ?? p.createdAt ?? ''),
    method: String(p.method ?? 'Manual'), note: String(p.note ?? ''),
  };
}

function discountFromApi(value: unknown) {
  const d = asRecord(value);
  const rawKind = String(d.kind ?? 'Percentage').toLowerCase();
  if (rawKind !== 'fixed' && rawKind !== 'percentage') throw new Error('The API returned an unsupported discount kind.');
  const kind: 'Fixed' | 'Percentage' = rawKind === 'fixed' ? 'Fixed' : 'Percentage';
  const endsAt = String(d.endsAt ?? '');
  return {
    id: String(d.id ?? crypto.randomUUID()), name: String(d.name ?? 'Discount'), kind,
    value: kind === 'Fixed' ? asMoney(d.value) : String(d.value ?? '0'),
    scope: String(d.scope ?? 'Subscriptions'), minimum: asMoney(d.minimumPurchase ?? d.minimum ?? '0.00'),
    minimumQuantity: Math.max(1, Number(d.minQuantity ?? d.minimumQuantity ?? 1)),
    usage: Number(d.useCount ?? d.usage ?? 0), limit: Number(d.maxUses ?? d.limit ?? 0),
    endsAt: endsAt.slice(0, 10), active: d.active !== false,
  };
}

function taxFromApi(value: unknown) {
  const tax = asRecord(value);
  return { id: String(tax.id ?? crypto.randomUUID()), name: String(tax.name ?? 'Tax'), region: String(tax.region ?? 'India'), rateBps: rateBasisPoints(tax.rate ?? tax.taxRate), active: tax.active !== false };
}

function staffFromApi(value: unknown) {
  const member = asRecord(value);
  const name = String(member.name ?? member.fullName ?? 'Team member');
  const role: 'Admin' | 'Staff' = String(member.role ?? 'staff').toLowerCase() === 'admin' ? 'Admin' : 'Staff';
  return { id: String(member.id ?? crypto.randomUUID()), name, email: String(member.email ?? ''), role, status: String(member.status ?? 'Active'), initials: name.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase() };
}

function exportFromApi(value: unknown) {
  const job = asRecord(value);
  const manifest = asRecord(job.manifest);
  const counts = asRecord(manifest.counts ?? job.counts);
  return {
    id: String(job.id ?? crypto.randomUUID()), name: String(job.name ?? 'Business data archive'),
    createdAt: String(job.createdAt ?? ''), status: String(job.status ?? 'Queued'),
    counts: Object.fromEntries(Object.entries(counts).map(([key, count]) => [key, Number(count)])),
  };
}

function approvalFromApi(value: unknown) {
  const approval = asRecord(value);
  const inputs = asRecord(approval.inputs);
  const rawStatus = String(approval.status ?? 'Pending').toLowerCase();
  const status: WorkspaceData['approvals'][number]['status'] = rawStatus === 'pending' || rawStatus === 'needs_review' ? 'Needs review' : rawStatus === 'approved' ? 'Approved' : rawStatus === 'declined' || rawStatus === 'rejected' ? 'Declined' : (() => { throw new Error('The API returned an unsupported approval status.'); })();
  const amount = inputs.amount;
  return {
    id: String(approval.id ?? crypto.randomUUID()), title: String(approval.title ?? approval.action ?? 'Review action'),
    detail: String(approval.detail ?? JSON.stringify(inputs)),
    ...(typeof amount === 'string' ? { amount: asMoney(amount) } : {}),
    target: String(approval.resourceId ?? approval.target ?? 'Workspace action'),
    createdAt: String(approval.createdAt ?? ''), status,
    action: String(approval.action ?? 'Review proposed action'),
    ...(Object.keys(asRecord(approval.executionResult)).length ? { executionResult: asRecord(approval.executionResult) } : {}),
  };
}

export function useWorkspace() {
  const [data, setData] = useState<WorkspaceData>(initialData);
  const [apiState, setApiState] = useState<ApiState>('checking');
  const [dataMode, setDataMode] = useState<DataMode>('checking');
  const [toast, setToast] = useState('');

  useEffect(() => {
    if (apiState !== 'offline' || dataMode !== 'demo') return;
    try { localStorage.setItem('orca-demo-v2', JSON.stringify(data)); } catch { /* Private browsing may block storage. */ }
  }, [apiState, dataMode, data]);

  useEffect(() => {
    let mounted = true;
    void apiAvailable().then(async (available) => {
      if (!mounted) return;
      setApiState(available ? 'online' : 'offline');
      if (!available) { setDataMode('demo'); return; }
      const [products, plans, customers, subscriptions, invoices, quotations, payments, discounts, taxes, staff, exports, approvals] = await Promise.all([
        readItemsResult<unknown>('/products'), readItemsResult<unknown>('/plans'), readItemsResult<unknown>('/customers'),
        readItemsResult<unknown>('/subscriptions'), readItemsResult<unknown>('/invoices'), readItemsResult<unknown>('/quotations'),
        readItemsResult<unknown>('/payments'), readItemsResult<unknown>('/discounts'), readItemsResult<unknown>('/taxes'),
        readItemsResult<unknown>('/staff'), readItemsResult<unknown>('/exports'), readItemsResult<unknown>('/agent/approvals'),
      ]);
      if (!mounted) return;
      const results = [products, plans, customers, subscriptions, invoices, quotations, payments, discounts, taxes, staff, exports, approvals];
      if (results.some((result) => result.status === 401)) {
        setDataMode('auth-required');
        return;
      }
      const hasWorkspaceData = results.some((result) => result.items !== null);
      if (!hasWorkspaceData) {
        setDataMode(results.some((result) => result.status === 403) ? 'auth-required' : 'demo');
        return;
      }
      setDataMode('workspace');
      const remoteProducts = safeMap(products.items, productFromApi) ?? [];
      const remotePlans = safeMap(plans.items, planFromApi) ?? [];
      const remoteCustomers = safeMap(customers.items, customerFromApi) ?? [];
      const remoteSubscriptions = safeMap(subscriptions.items, subscriptionFromApi) ?? [];
      const remoteInvoices = safeMap(invoices.items, invoiceFromApi) ?? [];
      const remoteQuotations = safeMap(quotations.items, quotationFromApi) ?? [];
      const remotePayments = safeMap(payments.items, paymentFromApi) ?? [];
      const remoteDiscounts = safeMap(discounts.items, discountFromApi) ?? [];
      const remoteTaxes = safeMap(taxes.items, taxFromApi) ?? [];
      const remoteStaff = safeMap(staff.items, staffFromApi) ?? [];
      const remoteExports = safeMap(exports.items, exportFromApi) ?? [];
      const remoteApprovals = safeMap(approvals.items, approvalFromApi) ?? [];
      setData((current) => ({
        ...current,
        products: remoteProducts,
        plans: remotePlans,
        customers: remoteCustomers,
        subscriptions: remoteSubscriptions,
        invoices: remoteInvoices,
        quotations: remoteQuotations,
        payments: remotePayments,
        discounts: remoteDiscounts,
        taxes: remoteTaxes,
        staff: remoteStaff,
        exports: remoteExports,
        approvals: remoteApprovals,
      }));
    });
    return () => { mounted = false; };
  }, []);

  const patch = useCallback(<K extends keyof WorkspaceData>(key: K, update: WorkspaceData[K] | ((current: WorkspaceData[K]) => WorkspaceData[K])) => {
    setData((current) => ({ ...current, [key]: typeof update === 'function' ? (update as (value: WorkspaceData[K]) => WorkspaceData[K])(current[key]) : update }));
  }, []);

  const notify = useCallback((message: string) => {
    setToast(message);
    window.setTimeout(() => setToast(''), 3400);
  }, []);

  const loadInvoice = useCallback(async (invoiceId: string): Promise<boolean> => {
    if (apiState !== 'online' || dataMode !== 'workspace') return false;
    try {
      const invoice = invoiceFromApi(await apiRequest<unknown>(`/invoices/${encodeURIComponent(invoiceId)}`));
      setData((current) => ({ ...current, invoices: [invoice, ...current.invoices.filter((item) => item.id !== invoice.id)] }));
      return true;
    } catch (error) {
      notify(error instanceof ApiError ? `Invoice could not be loaded: ${error.message}` : 'Invoice could not be loaded from the workspace.');
      return false;
    }
  }, [apiState, dataMode, notify]);

  const commit = useCallback(async (path: string, body: unknown, update: (response?: unknown) => void): Promise<boolean> => {
    if (apiState === 'offline') {
      update();
      notify('Saved in this browser demo');
      return true;
    }
    if (apiState !== 'online' || dataMode !== 'workspace') {
      notify(dataMode === 'auth-required' ? 'Sign in to save changes to this workspace. Demo records are read-only.' : 'Connect a signed-in API workspace to save this change.');
      return false;
    }
    try {
      const response = await post(path, body);
      update(response);
      notify('Saved to your workspace');
      return true;
    } catch (error) {
      notify(error instanceof ApiError ? `Not saved: ${error.message}` : 'Not saved. Check the API connection and try again.');
      return false;
    }
  }, [apiState, dataMode, notify]);

  return { data, setData, patch, apiState, dataMode, toast, notify, commit, loadInvoice };
}
