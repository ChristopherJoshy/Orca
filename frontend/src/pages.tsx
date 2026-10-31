import { useEffect, useState, type ReactNode } from 'react';
import { ApiError, apiRequest } from './api';
import { customerFor, isOverdue, money, outstanding, shortMoney, type Approval, type Invoice, type WorkspaceData, type Money, toPaise, addMoney, ratioMoney } from './model';
import { Avatar, Badge, Button, Card, EmptyState, PageHeading } from './ui';
import { Icon } from './icons';

export type PageKey = 'overview' | 'customers' | 'catalog' | 'plans' | 'quotations' | 'subscriptions' | 'invoices' | 'invoice' | 'payments' | 'reports' | 'settings' | 'discounts' | 'taxes' | 'staff' | 'exports' | 'copilot' | 'portal';
export type PageActions = {
  navigate: (page: PageKey, id?: string) => void;
  create: (kind: 'customer' | 'product' | 'plan' | 'quotation' | 'quoteTemplate' | 'subscription' | 'discount' | 'staff' | 'payment' | 'tax') => void;
  confirmInvoice: (invoice: Invoice) => void;
  cancelInvoice: (invoice: Invoice) => void;
  sendInvoice: (invoice: Invoice) => void;
  openPayment: (invoice: Invoice) => void;
  payOnline: (invoice: Invoice) => Promise<void>;
  transitionSubscription: (subscriptionId: string, action: string) => void;
  convertQuotation: (quotationId: string) => void;
  sendQuotation: (quotationId: string) => void;
  exportData: () => void;
  downloadExport: (jobId: string) => void;
  runBilling: () => Promise<number | null>;
  decideApproval: (approval: Approval, decision: 'approve' | 'reject') => void;
  search: string;
  setSearch: (value: string) => void;
};

const dateFmt = (value: string) => value ? new Date(`${value.slice(0, 10)}T12:00:00`).toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';
const toneFor = (status: string): 'neutral' | 'green' | 'amber' | 'red' | 'purple' | 'blue' => {
  if (['Active', 'Paid', 'Accepted', 'Approved', 'Completed'].includes(status)) return 'green';
  if (['Past due', 'Overdue', 'Declined', 'Failed'].includes(status)) return 'red';
  if (['Needs review', 'Draft', 'Quotation', 'Pending', 'Invited', 'Paused'].includes(status)) return 'amber';
  if (['Confirmed', 'Sent', 'Staff'].includes(status)) return 'blue';
  if (['Admin'].includes(status)) return 'purple';
  return 'neutral';
};

function TableWrap({ children }: { children: ReactNode }) { return <div className="table-scroll" role="group" tabIndex={0} aria-label="Scrollable table region"><table className="data-table">{children}</table></div>; }
function SearchBar({ placeholder, value, onChange }: { placeholder: string; value: string; onChange?: (value: string) => void }) {
  return <label className="table-search"><Icon name="search" size={16} /><span className="sr-only">Search</span><input value={value} onChange={(event) => onChange?.(event.target.value)} placeholder={placeholder} /></label>;
}
function TableToolbar({ title, count, search, setSearch, actions }: { title: string; count: string; search: string; setSearch?: (value: string) => void; actions?: ReactNode }) {
  return <div className="table-toolbar"><div className="table-title"><h2>{title}</h2><span className="count-pill">{count}</span></div><div className="toolbar-actions">{setSearch && <SearchBar value={search} onChange={setSearch} placeholder="Search records…" />}{actions}</div></div>;
}
function Status({ value }: { value: string }) { return <Badge tone={toneFor(value)}>{value}</Badge>; }
function CellMoney({ value, strong = false }: { value: Money; strong?: boolean }) { return <span className={`amount ${strong ? 'amount-strong' : ''}`}>{money(value)}</span>; }
function PayOnlineAction({ invoice, onPay, size = 'md' }: { invoice: Invoice; onPay: (invoice: Invoice) => Promise<void>; size?: 'sm' | 'md' }) {
  const [feedback, setFeedback] = useState<{ kind: 'pending' | 'error'; message: string } | null>(null);
  const pending = feedback?.kind === 'pending';
  const startPayment = async () => {
    setFeedback({ kind: 'pending', message: 'Opening secure Stripe Checkout…' });
    try {
      await onPay(invoice);
    } catch (error) {
      setFeedback({ kind: 'error', message: error instanceof Error ? error.message : 'Could not start online payment. Try again.' });
    }
  };
  return <div className={`online-payment-action ${size === 'sm' ? 'online-payment-action-compact' : ''}`}>
    <Button variant="primary" size={size} icon="card" disabled={pending} onClick={() => void startPayment()}>{pending ? 'Opening checkout…' : 'Pay online'}</Button>
    {feedback && <p className={`online-payment-feedback ${feedback.kind === 'error' ? 'online-payment-error' : ''}`} role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.message}</p>}
  </div>;
}
function marginPercent(price: Money, cost: Money): string {
  const denominator = toPaise(price);
  if (denominator <= 0n) return '—';
  return `${((denominator - toPaise(cost)) * 100n / denominator).toString()}%`;
}
function rateText(basisPoints: number): string {
  return `${Math.floor(basisPoints / 100)}.${String(Math.abs(basisPoints % 100)).padStart(2, '0')}%`;
}

function MetricCard({ label, value, note, icon, trend, tone = 'lavender' }: { label: string; value: string; note: string; icon: 'trend' | 'card' | 'users' | 'clock' | 'payments'; trend?: string; tone?: string }) {
  return <Card className="metric-card"><div className="metric-top"><span>{label}</span><span className={`metric-icon ${tone}`}><Icon name={icon} size={17} /></span></div><div className="metric-value">{value}</div><div className="metric-bottom">{trend && <span className="trend-pill"><Icon name="up" size={13} />{trend}</span>}<span>{note}</span></div></Card>;
}

function ChartCard({ data }: { data: WorkspaceData }) {
  const now = new Date();
  const monthRows = Array.from({ length: 6 }, (_, index) => {
    const date = new Date(now.getFullYear(), now.getMonth() - 5 + index, 1);
    const key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
    return {
      key,
      label: date.toLocaleDateString('en-IN', { month: 'short' }),
      collected: data.payments.filter((payment) => payment.date.startsWith(key)).reduce((sum, payment) => addMoney(sum, payment.amount), '0.00' as Money),
      invoiced: data.invoices.filter((invoice) => invoice.issueDate.startsWith(key) && !['Draft', 'Cancelled'].includes(invoice.status)).reduce((sum, invoice) => addMoney(sum, invoice.total), '0.00' as Money),
    };
  });
  const max = Math.max(1, ...monthRows.flatMap((row) => [Number(toPaise(row.collected)) / 100, Number(toPaise(row.invoiced)) / 100]));
  const yFor = (value: Money) => 228 - (Number(toPaise(value)) / 100 / max) * 188;
  const collectedPath = monthRows.map((row, index) => `${index === 0 ? 'M' : 'L'} ${40 + index * 128} ${yFor(row.collected)}`).join(' ');
  const invoicedPath = monthRows.map((row, index) => `${index === 0 ? 'M' : 'L'} ${40 + index * 128} ${yFor(row.invoiced)}`).join(' ');
  const area = `${collectedPath} L 680 240 L 40 240 Z`;
  const active = data.subscriptions.filter((sub) => sub.status === 'Active');
  const activeValue = active.reduce((sum, sub) => addMoney(sum, sub.amount), '0.00' as Money);
  return <Card className="revenue-card">
    <div className="card-heading"><div><div className="eyebrow">Revenue overview</div><h2>{shortMoney(activeValue)} <span className="heading-unit">total contracted</span></h2><p>Recurring value across {active.length} active subscriptions</p></div><span className="select-control static-control">Last 6 months <Icon name="calendar" size={15} /></span></div>
    <div className="chart-wrap"><div className="chart-y"><span>₹{Math.round(max / 1000)}K</span><span>₹{Math.round(max * .75 / 1000)}K</span><span>₹{Math.round(max * .5 / 1000)}K</span><span>₹{Math.round(max * .25 / 1000)}K</span><span>₹0</span></div><svg className="revenue-chart" viewBox="0 0 720 260" role="img" aria-label="Recorded invoices and payments over the last six months"><defs><linearGradient id="chart-fill" x1="0" x2="0" y1="0" y2="1"><stop offset="0%" stopColor="#9374dc" stopOpacity=".20" /><stop offset="100%" stopColor="#9374dc" stopOpacity="0" /></linearGradient></defs><g className="chart-grid"><path d="M30 40H710M30 90H710M30 140H710M30 190H710M30 240H710" /></g><path d={area} fill="url(#chart-fill)" /><path d={invoicedPath} fill="none" stroke="#c7c0d9" strokeWidth="2" strokeDasharray="4 5" strokeLinecap="round" strokeLinejoin="round" /><path d={collectedPath} fill="none" stroke="#8a6bd2" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />{monthRows.map((row, index) => <circle key={row.key} cx={40 + index * 128} cy={yFor(row.collected)} r="4" fill="#8a6bd2" stroke="white" strokeWidth="2" />)}</svg><div className="chart-x">{monthRows.map((row) => <span key={row.key}>{row.label}</span>)}</div></div>
    <div className="chart-legend"><span><i className="legend-dot purple" />Payments recorded</span><span><i className="legend-dot light" />Invoice value</span><strong>Stored records · 6-month window</strong></div>
  </Card>;
}

export function OverviewPage({ data, actions, canRunBilling }: { data: WorkspaceData; actions: PageActions; canRunBilling: boolean }) {
  const [billingBusy, setBillingBusy] = useState(false);
  const [billingMessage, setBillingMessage] = useState('');
  const runBilling = async () => {
    setBillingBusy(true);
    setBillingMessage('');
    const created = await actions.runBilling();
    setBillingBusy(false);
    if (created === null) return;
    setBillingMessage(created > 0 ? `Generated ${created} draft invoice${created === 1 ? '' : 's'}. Refreshing workspace…` : 'No subscriptions were due for an invoice.');
    if (created > 0) window.setTimeout(() => window.location.reload(), 800);
  };
  const hour = new Date().getHours();
  const greeting = hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
  const active = data.subscriptions.filter((sub) => sub.status === 'Active');
  const mrr = active.reduce((sum, sub) => {
    const plan = data.plans.find((item) => item.id === sub.planId);
    const monthly = plan?.cadence === 'Yearly' ? ratioMoney(sub.amount, 1, 12) : plan?.cadence === 'Weekly' ? ratioMoney(sub.amount, 52, 12) : plan?.cadence === 'Daily' ? ratioMoney(sub.amount, 365, 12) : sub.amount;
    return addMoney(sum, monthly);
  }, '0.00' as Money);
  const receivable = data.invoices.filter((invoice) => invoice.status === 'Confirmed').reduce((sum, invoice) => addMoney(sum, outstanding(invoice)), '0.00' as Money);
  const currentMonth = new Date().toISOString().slice(0, 7);
  const paymentsThisMonth = data.payments.filter((payment) => payment.date.slice(0, 7) === currentMonth);
  const paid = paymentsThisMonth.reduce((sum, payment) => addMoney(sum, payment.amount), '0.00' as Money);
  const overdue = data.invoices.filter(isOverdue);
  const recent = [...data.invoices].sort((a, b) => b.issueDate.localeCompare(a.issueDate)).slice(0, 4);
  const renewalWindow = active.filter((sub) => sub.renewsAt >= new Date().toISOString().slice(0, 10) && (new Date(`${sub.renewsAt}T12:00:00`).getTime() - Date.now()) / 86400000 <= 14).length;
  const draftInvoices = data.invoices.filter((invoice) => invoice.status === 'Draft').length;
  const openInvoices = data.invoices.filter((invoice) => invoice.status === 'Confirmed' && toPaise(outstanding(invoice)) > 0n).length;
  const paidInvoices = data.invoices.filter((invoice) => invoice.status === 'Paid').length;
  const invoiceCount = Math.max(1, data.invoices.length);
  const draftWidth = draftInvoices / invoiceCount * 100;
  const openWidth = openInvoices / invoiceCount * 100;
  const paidWidth = paidInvoices / invoiceCount * 100;
  return <>
    <PageHeading eyebrow={new Intl.DateTimeFormat('en-IN', { weekday: 'long', month: 'long', day: 'numeric' }).format(new Date())} title={greeting} description="Here’s what’s happening across your subscription business." actions={<><span className="select-control static-control">This month <Icon name="calendar" size={15} /></span>{canRunBilling && <Button variant="outline" icon="calendar" disabled={billingBusy} onClick={() => void runBilling()}>{billingBusy ? 'Running billing…' : 'Run billing'}</Button>}<Button variant="primary" icon="plus" onClick={() => actions.create('quotation')}>Create quotation</Button></>} />
    {billingMessage && <p className="billing-run-message" role="status">{billingMessage}</p>}
    <div className="metric-grid">
      <MetricCard label="Monthly recurring revenue" value={shortMoney(mrr)} note="Normalized from active plans" icon="trend" />
      <MetricCard label="Collected this month" value={shortMoney(paid)} note={`${paymentsThisMonth.length} recorded payments`} icon="card" tone="mint" />
      <MetricCard label="Active subscriptions" value={String(active.length).padStart(2, '0')} note={`${data.subscriptions.filter((s) => s.status === 'Quotation').length} awaiting confirmation`} icon="users" tone="sky" />
      <MetricCard label="Open receivables" value={shortMoney(receivable)} note={`${overdue.length} invoices overdue`} icon="clock" tone="peach" />
    </div>
    <div className="overview-grid"><ChartCard data={data} /><Card className="pipeline-card"><div className="card-heading"><div><div className="eyebrow">Billing control tower</div><h2>Billing pipeline</h2><p>Current invoice and renewal records</p></div><button type="button" className="more-button" aria-label="More billing pipeline actions"><Icon name="more" /></button></div><div className="pipeline-number"><strong>{String(renewalWindow).padStart(2, '0')}</strong><span>renewals in the next 14 days</span></div><div className="pipeline-progress"><div className="progress-head"><span>Invoice status mix</span><strong>{data.invoices.length} records</strong></div><div className="progress-track progress-stacked"><span className="segment-draft" style={{ width: `${draftWidth}%` }} /><span className="segment-open" style={{ width: `${openWidth}%` }} /><span className="segment-paid" style={{ width: `${paidWidth}%` }} /></div><div className="progress-meta"><span>{draftInvoices} draft · {openInvoices} open</span><span>{paidInvoices} paid</span></div></div><div className="pipeline-rows"><div><span className="pipeline-indicator queued" /><span>Subscriptions due soon</span><strong>{renewalWindow}</strong></div><div><span className="pipeline-indicator queued" /><span>Draft invoices</span><strong>{draftInvoices}</strong></div><div><span className="pipeline-indicator working" /><span>Open invoices</span><strong>{openInvoices}</strong></div><div><span className="pipeline-indicator done"><Icon name="check" size={12} /></span><span>Paid invoices</span><strong>{paidInvoices}</strong></div></div><button className="text-link" type="button" onClick={() => actions.navigate('reports')}>View billing health <Icon name="arrow" size={15} /></button></Card></div>
    <div className="lower-grid"><Card className="dashboard-recent-invoices"><TableToolbar title="Recent invoices" count={`${recent.length} recent`} search="" actions={<Button variant="quiet" size="sm" onClick={() => actions.navigate('invoices')}>View all <Icon name="arrow" size={15} /></Button>} /><TableWrap><thead><tr><th>Invoice</th><th>Customer</th><th>Issued</th><th>Status</th><th className="align-right">Amount</th></tr></thead><tbody>{recent.map((invoice) => <tr key={invoice.id}><td><button className="link-button" type="button" onClick={() => actions.navigate('invoice', invoice.id)}>{invoice.number}</button></td><td>{customerFor(data, invoice.customerId).company}</td><td>{dateFmt(invoice.issueDate)}</td><td><Status value={isOverdue(invoice) ? 'Overdue' : invoice.status} /></td><td className="align-right"><CellMoney value={invoice.total} strong /></td></tr>)}</tbody></TableWrap></Card><Card className="renewals-card"><TableToolbar title="Upcoming renewals" count="Next 14 days" search="" actions={<span className="count-pill">{data.subscriptions.filter((sub) => sub.status === 'Active').length} active</span>} /><div className="renewal-list">{data.subscriptions.filter((sub) => sub.status === 'Active').slice(0, 4).map((sub) => <div className="renewal-row" key={sub.id}><Avatar initials={customerFor(data, sub.customerId).initials} size="sm" /><div className="renewal-customer"><strong>{customerFor(data, sub.customerId).company}</strong><span>{sub.number} · {sub.seats} seats</span></div><div className="renewal-date"><strong>{dateFmt(sub.renewsAt)}</strong><span>Renews</span></div><CellMoney value={sub.amount} strong /></div>)}</div><button className="text-link renewal-link" type="button" onClick={() => actions.navigate('subscriptions')}>Manage subscriptions <Icon name="arrow" size={15} /></button></Card></div>
  </>;
}

export function CustomersPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.customers.filter((customer) => `${customer.name} ${customer.company} ${customer.email}`.toLowerCase().includes(actions.search.toLowerCase()));
  return <><PageHeading eyebrow="Relationships" title="Customers" description="Keep every customer relationship, subscription, and invoice in one place." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('customer')}>Add customer</Button>} /><Card className="customer-list-card"><TableToolbar title="All customers" count={`${rows.length} customers`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Customer</th><th>Company</th><th>Subscriptions</th><th>Customer since</th><th>Status</th><th className="align-right">Open balance</th><th><span className="sr-only">Customer actions</span></th></tr></thead><tbody>{rows.map((customer) => { const subs = data.subscriptions.filter((s) => s.customerId === customer.id && s.status === 'Active'); const balance = data.invoices.filter((i) => i.customerId === customer.id && i.status === 'Confirmed').reduce((sum, i) => addMoney(sum, outstanding(i)), '0.00' as Money); return <tr key={customer.id}><td><div className="person-cell"><Avatar initials={customer.initials} size="sm" /><span><strong>{customer.name}</strong><small>{customer.email}</small></span></div></td><td>{customer.company}</td><td><span className="table-strong">{subs.length}</span> active</td><td>{dateFmt(customer.since)}</td><td><Status value={balance !== '0.00' && customer.status === 'Past due' ? 'Past due' : customer.status} /></td><td className="align-right"><CellMoney value={balance} /></td><td><button type="button" className="icon-button" aria-label={`View ${customer.company}`} onClick={() => actions.navigate('subscriptions')}> <Icon name="arrow" size={16} /></button></td></tr>; })}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="customers" title="No customers match" description="Try another name or company." />}</Card></>;
}

export function CatalogPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.products.filter((product) => `${product.name} ${product.type} ${product.notes}`.toLowerCase().includes(actions.search.toLowerCase()));
  return <><PageHeading eyebrow="Offerings" title="Catalog" description="Your products and services, with flexible variants and clear pricing." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('product')}>Add product</Button>} /><div className="catalog-summary"><Card className="catalog-intro"><div className="catalog-icon"><Icon name="catalog" size={22} /></div><div><h2>Product catalog</h2><p>Products are the building blocks of your plans. Variant adjustments are added to the base price.</p></div></Card><Card className="catalog-stat"><span>Active products</span><strong>{data.products.filter((p) => p.active).length}</strong></Card><Card className="catalog-stat"><span>Active variants</span><strong>{data.products.reduce((sum, p) => sum + p.variants.length, 0)}</strong></Card></div><Card className="catalog-list-card"><TableToolbar title="All products" count={`${rows.length} products`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Product</th><th>Type</th><th>Variants</th><th>Sales price</th><th>Cost price</th><th>Margin</th><th>Tax rate</th><th>Status</th><th><span className="sr-only">Product actions</span></th></tr></thead><tbody>{rows.map((product) => <tr key={product.id}><td><div className="product-cell"><span className="product-thumb"><Icon name="catalog" size={17} /></span><span><strong>{product.name}</strong><small>{product.notes}</small></span></div></td><td>{product.type}</td><td>{product.variants.length ? product.variants.map((v) => <span className="variant-chip" key={v.name}>{v.name} {toPaise(v.adjustment) > 0n && <small>+{money(v.adjustment)}</small>}</span>) : <span className="muted">—</span>}</td><td><CellMoney value={product.price} strong /></td><td><CellMoney value={product.cost} /></td><td><span className="margin-pill">{marginPercent(product.price, product.cost)}</span></td><td>{rateText(product.taxRateBps)}</td><td><Status value={product.active ? 'Active' : 'Paused'} /></td><td><button type="button" className="icon-button" aria-label={`Edit ${product.name}`}><Icon name="more" /></button></td></tr>)}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="catalog" title="No products found" description="Try a different search or add your first product." />}</Card></>;
}

export function PlansPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.plans.filter((plan) => `${plan.name} ${plan.cadence}`.toLowerCase().includes(actions.search.toLowerCase()));
  return <><PageHeading eyebrow="Billing setup" title="Plans" description="Set billing frequency, availability, and lifecycle rules for each plan." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('plan')}>Create plan</Button>} /><div className="plan-grid">{rows.map((plan) => <Card className="plan-card" key={plan.id}><div className="plan-card-top"><div className="plan-mark"><Icon name="plans" size={19} /></div><Status value={plan.active ? 'Active' : 'Paused'} /></div><div className="eyebrow">{plan.cadence} billing</div><h2>{plan.name}</h2><div className="plan-price"><strong>{money(plan.price)}</strong><span>/{plan.cadence === 'Yearly' ? 'year' : plan.cadence === 'Monthly' ? 'month' : plan.cadence === 'Weekly' ? 'week' : 'day'}</span></div><div className="plan-detail"><span>Minimum quantity</span><strong>{plan.minimumQuantity} seats</strong></div><div className="plan-detail"><span>Tax rate</span><strong>{rateText(plan.taxRateBps)}</strong></div><div className="plan-detail"><span>Available since</span><strong>{dateFmt(plan.availableFrom)}</strong></div><div className="plan-policies">{plan.pauseAllowed && <span>Pause</span>}{plan.closeAllowed && <span>Early close</span>}{plan.renewAllowed && <span>Renew</span>}</div><button type="button" className="text-link" onClick={() => actions.navigate('subscriptions')}>View subscriptions <Icon name="arrow" size={15} /></button></Card>)}</div>{rows.length === 0 && <EmptyState icon="plans" title="No plans found" description="Create a plan to offer recurring products and services." action={<Button variant="primary" onClick={() => actions.create('plan')}>Create your first plan</Button>} />}</>;
}

export function SubscriptionsPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.subscriptions.filter((sub) => `${sub.number} ${customerFor(data, sub.customerId).company} ${sub.status}`.toLowerCase().includes(actions.search.toLowerCase()));
  const statusCounts = ['Active', 'Quotation', 'Paused', 'Closed'].map((status) => [status, data.subscriptions.filter((s) => s.status === status).length] as const);
  return <><PageHeading eyebrow="Recurring revenue" title="Subscriptions" description="Track every agreement from draft through its full lifecycle." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('subscription')}>New subscription</Button>} /><div className="compact-stats">{statusCounts.map(([label, count]) => <Card key={label}><span>{label}</span><strong>{String(count).padStart(2, '0')}</strong></Card>)}</div><Card><TableToolbar title="All subscriptions" count={`${rows.length} records`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Subscription</th><th>Customer</th><th>Plan</th><th>Seats</th><th>Starts</th><th>Renews</th><th>Status</th><th className="align-right">Recurring amount</th><th>Next action</th></tr></thead><tbody>{rows.map((sub) => { const plan = data.plans.find((p) => p.id === sub.planId); const transition = sub.status === 'Draft' ? 'Request quote' : sub.status === 'Quotation' ? 'Confirm' : sub.status === 'Confirmed' ? 'Activate' : sub.status === 'Paused' ? 'Resume' : 'Manage'; return <tr key={sub.id}><td><strong className="linkish">{sub.number}</strong><small className="cell-subtitle">{sub.paymentTerms}</small></td><td><div className="person-cell"><Avatar initials={customerFor(data, sub.customerId).initials} size="sm" /><span><strong>{customerFor(data, sub.customerId).company}</strong><small>{customerFor(data, sub.customerId).name}</small></span></div></td><td>{plan?.name ?? 'Plan'}</td><td>{sub.seats}</td><td>{dateFmt(sub.startedAt)}</td><td>{dateFmt(sub.renewsAt)}</td><td><Status value={sub.status} /></td><td className="align-right"><CellMoney value={sub.amount} strong /></td><td><button className="small-action" type="button" onClick={() => actions.transitionSubscription(sub.id, transition)}>{transition}</button></td></tr>; })}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="subscriptions" title="No subscriptions match" description="Change your search or start a new subscription." />}</Card></>;
}

export function InvoicesPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const [statusFilter, setStatusFilter] = useState('All');
  const rows = data.invoices.filter((invoice) => {
    const textMatches = `${invoice.number} ${customerFor(data, invoice.customerId).company} ${invoice.status}`.toLowerCase().includes(actions.search.toLowerCase());
    const statusMatches = statusFilter === 'All' || (statusFilter === 'Overdue' ? isOverdue(invoice) : invoice.status === statusFilter);
    return textMatches && statusMatches;
  });
  const open = data.invoices.filter((invoice) => invoice.status === 'Confirmed' && toPaise(outstanding(invoice)) > 0n).length;
  const overdue = data.invoices.filter(isOverdue).length;
  return <><PageHeading eyebrow="Billing" title="Invoices" description="Review billing documents, balances, and stored calculation evidence." actions={<Button variant="outline" icon="download" onClick={() => actions.navigate('exports')}>Export</Button>} /><div className="compact-stats"><Card><span>Total this period</span><strong>{data.invoices.length}</strong></Card><Card><span>Awaiting payment</span><strong>{open}</strong></Card><Card><span>Overdue</span><strong className={overdue ? 'stat-danger' : ''}>{overdue}</strong></Card></div><Card className="invoice-list-card"><TableToolbar title="All invoices" count={`${rows.length} records`} search={actions.search} setSearch={actions.setSearch} actions={<label className="select-control">Status <select aria-label="Filter invoices by status" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="All">All statuses</option><option value="Draft">Draft</option><option value="Confirmed">Confirmed</option><option value="Paid">Paid</option><option value="Overdue">Overdue</option><option value="Cancelled">Cancelled</option></select><Icon name="chevron" size={15} /></label>} /><TableWrap><thead><tr><th>Invoice</th><th>Customer</th><th>Billing period</th><th>Due date</th><th>Status</th><th>Balance</th><th className="align-right">Total</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{rows.map((invoice) => { const balance = invoice.status === 'Confirmed' ? outstanding(invoice) : '0.00' as Money; const overdueStatus = isOverdue(invoice); return <tr key={invoice.id}><td><button className="link-button" type="button" onClick={() => actions.navigate('invoice', invoice.id)}>{invoice.number}</button><small className="cell-subtitle">Issued {dateFmt(invoice.issueDate)}</small></td><td><div className="person-cell"><Avatar initials={customerFor(data, invoice.customerId).initials} size="sm" /><span><strong>{customerFor(data, invoice.customerId).company}</strong><small>{customerFor(data, invoice.customerId).name}</small></span></div></td><td>{invoice.period}</td><td className={overdueStatus ? 'text-danger' : ''}>{dateFmt(invoice.dueDate)}</td><td><Status value={overdueStatus ? 'Overdue' : invoice.status} /></td><td><CellMoney value={balance} /></td><td className="align-right"><CellMoney value={invoice.total} strong /></td><td><button type="button" className="small-action" onClick={() => actions.navigate('invoice', invoice.id)}>Invoice Truth</button></td></tr>; })}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="invoices" title="No invoices match" description="Try another search or review your filters." />}</Card></>;
}

export function InvoiceTruthPage({ data, invoiceId, businessName, actions, isCustomer, canManageInvoices, canPayOnline }: { data: WorkspaceData; invoiceId: string; businessName: string; actions: PageActions; isCustomer: boolean; canManageInvoices: boolean; canPayOnline: boolean }) {
  const invoice = data.invoices.find((item) => item.id === invoiceId) ?? data.invoices.find((item) => item.number === invoiceId);
  if (!invoice) return <><PageHeading title="Invoice not found" description="This invoice may have been removed or is outside your workspace." actions={<Button variant="outline" onClick={() => actions.navigate('invoices')}>Back to invoices</Button>} /><EmptyState icon="invoices" title="No invoice evidence available" description="Return to invoices and choose a record in this workspace." /></>;
  const customer = customerFor(data, invoice.customerId);
  const snapshot = invoice.snapshot;
  const balance = invoice.status === 'Confirmed' ? outstanding(invoice) : '0.00' as Money;
  const linesMatch = Boolean(snapshot && snapshot.subtotal === invoice.subtotal && snapshot.discountTotal === invoice.discount && snapshot.taxTotal === invoice.tax && snapshot.total === invoice.total);
  const markPaid = invoice.status === 'Paid';
  const formatRate = (rate: string) => `${rate.replace(/(?:\.0+|(\.\d*?[1-9])0+)$/, '$1')}%`;
  return <>
    <div className="detail-back"><button className="back-link" type="button" onClick={() => actions.navigate('invoices')}><Icon name="arrow" size={15} /> Back to invoices</button><span>/</span><span>{invoice.number}</span></div>
    <PageHeading eyebrow="Invoice Truth · stored evidence" title={invoice.number} description={`${customer.company} · ${invoice.period}`} actions={<><Button variant="outline" icon="print" onClick={() => window.print()}>Print</Button>{invoice.status === 'Draft' && canManageInvoices && <Button variant="primary" icon="check" onClick={() => actions.confirmInvoice(invoice)}>Confirm invoice</Button>}{invoice.status === 'Confirmed' && toPaise(balance) > 0n && (isCustomer ? canPayOnline && <PayOnlineAction key={invoice.id} invoice={invoice} onPay={actions.payOnline} /> : canManageInvoices && <><Button variant="outline" icon="mail" onClick={() => actions.sendInvoice(invoice)}>Send invoice</Button><Button variant="outline" onClick={() => actions.cancelInvoice(invoice)}>Cancel</Button><Button variant="primary" icon="plus" onClick={() => actions.openPayment(invoice)}>Record payment</Button></>)}</>} />
    <div className="invoice-detail-grid"><div className="invoice-main-column"><Card className="invoice-document"><div className="invoice-document-head"><div className="document-brand"><span className="brand-mark compact"><Icon name="overview" size={17} /></span><div><strong>{businessName}</strong><small>Invoice · {invoice.number}</small></div></div><Status value={isOverdue(invoice) ? 'Overdue' : invoice.status} /></div><div className="document-addresses"><div><span className="eyebrow">Billed to</span><strong>{customer.company}</strong><span>{customer.name}</span><span>{customer.email}</span></div><div><span className="eyebrow">Invoice details</span><span>Issued {dateFmt(invoice.issueDate)}</span><span>Due {dateFmt(invoice.dueDate)}</span><span>Period {invoice.period}</span></div></div>{snapshot?.lines.length ? <TableWrap><thead><tr><th>Description</th><th className="align-right">Qty</th><th className="align-right">Unit price</th><th className="align-right">Tax</th><th className="align-right">Line total</th></tr></thead><tbody>{snapshot.lines.map((line, index) => <tr key={`${line.description}-${index}`}><td><strong>{line.description}{line.variant ? ` · ${line.variant}` : ''}</strong>{line.discountAmount !== '0.00' && <small className="cell-subtitle">Discount {money(line.discountAmount)}</small>}</td><td className="align-right">{line.quantity}</td><td className="align-right"><CellMoney value={line.unitPrice} /></td><td className="align-right">{formatRate(line.taxRate)}</td><td className="align-right"><CellMoney value={line.lineTotal} strong /></td></tr>)}</tbody></TableWrap> : <p className="empty-inline">No stored line calculation was returned for this invoice.</p>}<div className="invoice-totals"><div><span>Subtotal</span><CellMoney value={invoice.subtotal} /></div><div><span>Discount</span><CellMoney value={invoice.discount} /></div><div><span>Tax · after discount</span><CellMoney value={invoice.tax} /></div><div className="total-row"><strong>Total due</strong><CellMoney value={invoice.total} strong /></div><div className="balance-row"><span>{invoice.status === 'Draft' ? 'Due when confirmed' : markPaid ? 'Paid in full' : 'Outstanding balance'}</span><CellMoney value={balance} strong /></div></div></Card><Card className="calculation-card"><div className="card-heading"><div><div className="eyebrow">Calculation snapshot</div><h2>Stored invoice evidence</h2><p>{snapshot ? 'Values and explanations below come from the immutable snapshot returned with this invoice.' : 'The API did not return a stored calculation snapshot.'}</p></div><Badge tone={!snapshot ? 'neutral' : linesMatch ? 'green' : 'amber'}>{!snapshot ? 'Unavailable' : linesMatch ? 'Matches invoice' : 'Totals differ'}</Badge></div>{snapshot ? <><div className="calc-steps"><div><span className="calc-step-num">01</span><span>Stored subtotal</span><strong>{money(snapshot.subtotal)}</strong></div><div><span className="calc-step-num">02</span><span>Stored discount</span><strong>− {money(snapshot.discountTotal)}</strong></div><div><span className="calc-step-num">03</span><span>Stored tax</span><strong>+ {money(snapshot.taxTotal)}</strong></div><div className="calc-final"><span className="calc-step-num">04</span><span>Stored invoice total</span><strong>{money(snapshot.total)}</strong></div></div><div className="snapshot-meta"><span>Engine version</span><strong>{snapshot.engineVersion || 'Not supplied'}</strong><span>Calculation hash</span><code>{invoice.calculationHash || 'No canonical hash supplied'}</code><span>Currency</span><strong>{snapshot.currency}</strong></div><ol className="snapshot-explanations">{snapshot.explanation.map((step, index) => <li key={`${index}-${step}`}>{step}</li>)}</ol>{snapshot.lines.map((line, index) => <details className="snapshot-line-detail" key={`${line.productId ?? line.description}-${index}`}><summary>{line.description} · stored line explanation</summary><ol>{line.explanation.map((step, stepIndex) => <li key={`${stepIndex}-${step}`}>{step}</li>)}</ol></details>)}</> : <p className="empty-inline">Invoice totals remain available above, but no calculation snapshot or canonical hash is attached to this record.</p>}</Card></div><aside className="invoice-side-column"><Card className="balance-card"><div className="eyebrow">Payment status</div><h2>{money(balance)}</h2><p>{invoice.status === 'Draft' ? 'This draft is not yet a receivable.' : invoice.status === 'Cancelled' ? 'This invoice was cancelled.' : markPaid ? 'This invoice has been fully allocated.' : isOverdue(invoice) ? `Past due since ${dateFmt(invoice.dueDate)}.` : 'Remaining balance on this invoice.'}</p><div className="balance-divider" /><div><span>Invoice total</span><CellMoney value={invoice.total} /></div><div><span>Payments received</span><CellMoney value={invoice.paid} /></div>{invoice.status === 'Confirmed' && toPaise(balance) > 0n && canManageInvoices && <Button variant="primary" className="full-width" onClick={() => actions.openPayment(invoice)}>Record a payment</Button>}</Card><Card><div className="card-heading small-heading"><div><div className="eyebrow">Payment activity</div><h2>Allocations</h2></div><Icon name="card" size={18} /></div>{data.payments.filter((payment) => payment.invoiceId === invoice.id).length === 0 ? <p className="empty-inline">No payments allocated yet.</p> : <div className="allocation-list">{data.payments.filter((payment) => payment.invoiceId === invoice.id).map((payment) => <div key={payment.id}><span className="payment-icon"><Icon name="check" size={15} /></span><span><strong>{payment.method}</strong><small>{dateFmt(payment.date)} · {payment.note || 'Payment allocation'}</small></span><CellMoney value={payment.amount} /></div>)}</div>}<div className="balance-divider" /><div className="customer-mini"><Avatar initials={customer.initials} size="sm" /><span><strong>{customer.company}</strong><small>{customer.name} · {customer.email}</small></span></div></Card><Card className="audit-mini"><div className="eyebrow">Audit trail</div><div className="audit-row"><span className="audit-dot" /><span><strong>{invoice.status === 'Paid' ? 'Payment allocation completed' : `${invoice.status} invoice`}</strong><small>{dateFmt(invoice.issueDate)} · Invoice record</small></span></div><div className="audit-row"><span className="audit-dot muted-dot" /><span><strong>{snapshot ? 'Calculation snapshot returned' : 'No calculation snapshot returned'}</strong><small>{snapshot ? `${snapshot.engineVersion || 'Engine version unavailable'} · ${invoice.calculationHash || 'hash unavailable'}` : 'No engine version or hash attached'}</small></span></div></Card></aside></div>
  </>;
}

export function QuotationsPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.quotations.filter((quote) => `${quote.number} ${quote.title} ${customerFor(data, quote.customerId).company}`.toLowerCase().includes(actions.search.toLowerCase()));
  return <>
    <PageHeading eyebrow="Sales workflow" title="Quotations" description="Build a clear offer from a reusable template and keep its validity visible." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('quotation')}>New quotation</Button>} />
    <div className="compact-stats"><Card><span>Open quotations</span><strong>{data.quotations.filter((q) => ['Draft', 'Sent'].includes(q.status)).length}</strong></Card><Card><span>Awaiting response</span><strong>{data.quotations.filter((q) => q.status === 'Sent').length}</strong></Card><Card><span>Accepted</span><strong>{data.quotations.filter((q) => q.status === 'Accepted').length}</strong></Card></div>
    <Card className="quotation-list-card">
      <TableToolbar title="Saved templates" count={`${data.quoteTemplates.length} ${data.quoteTemplates.length === 1 ? 'template' : 'templates'}`} search={actions.search} actions={<Button variant="outline" size="sm" icon="plus" onClick={() => actions.create('quoteTemplate')}>Create template</Button>} />
      {data.quoteTemplates.length === 0 ? <p className="empty-inline">Save a plan and optional product line to prepare consistent customer offers.</p> : <TableWrap><thead><tr><th>Template</th><th>Billing plan</th><th>Lines</th><th>Offer validity</th></tr></thead><tbody>{data.quoteTemplates.map((template) => <tr key={template.id}><td><strong>{template.name}</strong></td><td>{data.plans.find((plan) => plan.id === template.planId)?.name ?? 'Plan unavailable'}</td><td>{template.lines.map((line) => `${line.name} × ${line.quantity}`).join(' · ')}</td><td>{template.validDays} days</td></tr>)}</tbody></TableWrap>}
    </Card>
    <Card className="quotation-list-card"><TableToolbar title="All quotations" count={`${rows.length} quotes`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Quotation</th><th>Customer</th><th>Offer</th><th>Created</th><th>Valid through</th><th>Status</th><th className="align-right">Amount</th><th><span className="sr-only">Quotation actions</span></th></tr></thead><tbody>{rows.map((quote) => <tr key={quote.id}><td><strong className="linkish">{quote.number}</strong></td><td><div className="person-cell"><Avatar initials={customerFor(data, quote.customerId).initials} size="sm" /><span><strong>{customerFor(data, quote.customerId).company}</strong><small>{customerFor(data, quote.customerId).name}</small></span></div></td><td>{quote.title}</td><td>{dateFmt(quote.date)}</td><td>{dateFmt(quote.validUntil)}</td><td><Status value={quote.status} /></td><td className="align-right"><CellMoney value={quote.amount} strong /></td><td>{quote.status === 'Accepted' ? <button type="button" className="small-action" onClick={() => actions.convertQuotation(quote.id)}>Create subscription</button> : quote.status === 'Draft' ? <button type="button" className="small-action" onClick={() => actions.sendQuotation(quote.id)}>Send quotation</button> : <span className="muted">—</span>}</td></tr>)}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="quotations" title="No quotations yet" description="Prepare an offer for a customer in a few steps." action={<Button variant="primary" onClick={() => actions.create('quotation')}>Create quotation</Button>} />}</Card>
  </>;
}

export function PaymentsPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.payments.filter((payment) => `${payment.method} ${customerFor(data, payment.customerId).company} ${payment.note}`.toLowerCase().includes(actions.search.toLowerCase()));
  const collected = rows.reduce((sum, payment) => addMoney(sum, payment.amount), '0.00' as Money);
  return <><PageHeading eyebrow="Collections" title="Payments" description="Manual allocations and provider payments reconciled against invoices." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('payment')}>Record payment</Button>} /><div className="compact-stats"><Card><span>Recorded payments</span><strong>{rows.length}</strong></Card><Card><span>Total allocated</span><strong className="small-stat-money">{money(collected)}</strong></Card><Card><span>Payment methods</span><strong>{new Set(data.payments.map((p) => p.method)).size}</strong></Card></div><Card><TableToolbar title="Payment activity" count={`${rows.length} records`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Payment</th><th>Customer</th><th>Invoice</th><th>Method</th><th>Date</th><th>Note</th><th className="align-right">Amount allocated</th></tr></thead><tbody>{rows.map((payment) => { const invoice = data.invoices.find((i) => i.id === payment.invoiceId); return <tr key={payment.id}><td><div className="payment-id"><span className="payment-icon"><Icon name="check" size={15} /></span><strong>{payment.id.toUpperCase()}</strong></div></td><td>{customerFor(data, payment.customerId).company}</td><td><button type="button" className="link-button" onClick={() => invoice && actions.navigate('invoice', invoice.id)}>{invoice?.number ?? '—'}</button></td><td><span className="method-chip">{payment.method}</span></td><td>{dateFmt(payment.date)}</td><td>{payment.note || '—'}</td><td className="align-right"><CellMoney value={payment.amount} strong /></td></tr>; })}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="payments" title="No payments found" description="Record a payment allocation or adjust the search." />}</Card></>;
}

type ReportKind = 'Revenue' | 'Invoicing' | 'Collections' | 'Receivables';
type ReportPeriod = 'All time' | 'This month' | 'This quarter' | 'This year';
type ReportBar = { label: string; display: string; value: bigint; max: bigint };

function reportPeriodStart(period: ReportPeriod, now: Date): string | null {
  if (period === 'All time') return null;
  const year = now.getUTCFullYear();
  const month = now.getUTCMonth();
  const startMonth = period === 'This month' ? month : period === 'This quarter' ? Math.floor(month / 3) * 3 : 0;
  return new Date(Date.UTC(year, startMonth, 1)).toISOString().slice(0, 10);
}

function reportBarPercent(bar: ReportBar): string {
  if (bar.max <= 0n || bar.value <= 0n) return '0%';
  const basisPoints = (bar.value * 10000n) / bar.max;
  return String(Number(basisPoints > 10000n ? 10000n : basisPoints) / 100) + '%';
}

export function ReportsPage({ data }: { data: WorkspaceData }) {
  const [period, setPeriod] = useState<ReportPeriod>('All time');
  const [report, setReport] = useState<ReportKind>('Revenue');
  const now = new Date();
  const today = now.toISOString().slice(0, 10);
  const rangeStart = reportPeriodStart(period, now);
  const inRange = (date: string) => {
    const normalized = date.slice(0, 10);
    return Boolean(normalized) && (!rangeStart || normalized >= rangeStart) && (period === 'All time' || normalized <= today);
  };
  const invoices = data.invoices.filter((invoice) => inRange(invoice.issueDate));
  const payments = data.payments.filter((payment) => inRange(payment.date));
  const active = data.subscriptions.filter((subscription) => subscription.status === 'Active');
  const receivableInvoices = invoices.filter((invoice) => invoice.status === 'Confirmed' && toPaise(outstanding(invoice)) > 0n);
  const overdue = receivableInvoices.filter(isOverdue);
  const notYetDue = receivableInvoices.filter((invoice) => !isOverdue(invoice));
  const issuedInvoices = invoices.filter((invoice) => invoice.status !== 'Draft');
  const billableInvoices = invoices.filter((invoice) => invoice.status === 'Confirmed' || invoice.status === 'Paid');
  const draftInvoices = invoices.filter((invoice) => invoice.status === 'Draft');
  const paidInvoices = invoices.filter((invoice) => invoice.status === 'Paid');
  const cancelledInvoices = invoices.filter((invoice) => invoice.status === 'Cancelled');
  const invoicedTotal = billableInvoices.reduce((sum, invoice) => addMoney(sum, invoice.total), '0.00' as Money);
  const collectedTotal = payments.reduce((sum, payment) => addMoney(sum, payment.amount), '0.00' as Money);
  const openTotal = receivableInvoices.reduce((sum, invoice) => addMoney(sum, outstanding(invoice)), '0.00' as Money);
  const overdueTotal = overdue.reduce((sum, invoice) => addMoney(sum, outstanding(invoice)), '0.00' as Money);
  const notYetDueTotal = notYetDue.reduce((sum, invoice) => addMoney(sum, outstanding(invoice)), '0.00' as Money);
  const monthlyRecurringRevenue = active.reduce((sum, subscription) => {
    const cadence = data.plans.find((plan) => plan.id === subscription.planId)?.cadence;
    const monthly = cadence === 'Yearly' ? ratioMoney(subscription.amount, 1, 12) : cadence === 'Weekly' ? ratioMoney(subscription.amount, 52, 12) : cadence === 'Daily' ? ratioMoney(subscription.amount, 365, 12) : subscription.amount;
    return addMoney(sum, monthly);
  }, '0.00' as Money);
  const paymentInvoices = new Set(payments.map((payment) => payment.invoiceId));
  const byMethod = payments.reduce<Record<string, Money>>((totals, payment) => {
    totals[payment.method] = addMoney(totals[payment.method] ?? '0.00', payment.amount);
    return totals;
  }, {});
  const largestMoney = (values: Money[]) => values.reduce((largest, value) => toPaise(value) > toPaise(largest) ? value : largest, '0.00' as Money);
  const largestCount = (values: number[]) => Math.max(1, ...values);
  const moneyBar = (label: string, value: Money, max: Money): ReportBar => ({ label, display: shortMoney(value), value: toPaise(value), max: toPaise(max) });
  const countBar = (label: string, value: number, max: number): ReportBar => ({ label, display: String(value), value: BigInt(value), max: BigInt(max) });

  let metrics: { label: string; value: string; note: string; icon: 'trend' | 'card' | 'users' | 'clock' | 'payments'; tone?: string }[];
  let bars: ReportBar[];
  let insight: { eyebrow: string; title: string; body: string; footerLabel: string; footerValue: string };
  if (report === 'Revenue') {
    const max = largestMoney([monthlyRecurringRevenue, invoicedTotal, collectedTotal]);
    metrics = [
      { label: 'Monthly recurring revenue', value: shortMoney(monthlyRecurringRevenue), note: String(active.length) + ' active subscriptions · current run rate', icon: 'trend' },
      { label: 'Invoiced in period', value: shortMoney(invoicedTotal), note: String(billableInvoices.length) + ' confirmed or paid invoices', icon: 'payments', tone: 'peach' },
      { label: 'Collected in period', value: shortMoney(collectedTotal), note: String(payments.length) + ' payment allocations', icon: 'card', tone: 'mint' },
      { label: 'Invoices issued', value: String(issuedInvoices.length), note: 'From ' + period.toLowerCase(), icon: 'users', tone: 'sky' },
    ];
    bars = [moneyBar('Current monthly run rate', monthlyRecurringRevenue, max), moneyBar('Invoiced', invoicedTotal, max), moneyBar('Collected', collectedTotal, max)];
    insight = { eyebrow: 'Revenue summary', title: String(active.length) + ' active subscriptions', body: 'Monthly recurring revenue is normalized by plan cadence. The selected period contains ' + billableInvoices.length + ' billable invoices and ' + payments.length + ' recorded payment allocations.', footerLabel: 'Collected in period', footerValue: money(collectedTotal) };
  } else if (report === 'Invoicing') {
    const max = largestCount([invoices.length, draftInvoices.length, billableInvoices.length, paidInvoices.length]);
    metrics = [
      { label: 'Invoices issued', value: String(issuedInvoices.length), note: String(invoices.length) + ' total records in period', icon: 'payments' },
      { label: 'Invoiced amount', value: shortMoney(invoicedTotal), note: 'Confirmed and paid invoices', icon: 'trend', tone: 'mint' },
      { label: 'Draft invoices', value: String(draftInvoices.length), note: 'Not yet finalized', icon: 'clock', tone: 'peach' },
      { label: 'Paid invoices', value: String(paidInvoices.length), note: 'Fully allocated and settled', icon: 'card', tone: 'sky' },
    ];
    bars = [countBar('Draft', draftInvoices.length, max), countBar('Confirmed', invoices.filter((invoice) => invoice.status === 'Confirmed').length, max), countBar('Paid', paidInvoices.length, max), countBar('Cancelled', cancelledInvoices.length, max)];
    insight = { eyebrow: 'Invoice lifecycle', title: draftInvoices.length ? String(draftInvoices.length) + ' drafts need review' : 'No draft invoices in this period', body: String(issuedInvoices.length) + ' invoices were issued in the selected period; ' + paidInvoices.length + ' are paid and ' + receivableInvoices.length + ' still have an outstanding balance.', footerLabel: 'Confirmed and paid value', footerValue: money(invoicedTotal) };
  } else if (report === 'Collections') {
    const average = payments.length ? ratioMoney(collectedTotal, 1, payments.length) : '0.00';
    const methods = Object.entries(byMethod).sort((left, right) => toPaise(right[1]) > toPaise(left[1]) ? 1 : toPaise(right[1]) < toPaise(left[1]) ? -1 : left[0].localeCompare(right[0]));
    const max = largestMoney(methods.map(([, value]) => value));
    metrics = [
      { label: 'Collected in period', value: shortMoney(collectedTotal), note: String(payments.length) + ' allocations received', icon: 'card', tone: 'mint' },
      { label: 'Payment allocations', value: String(payments.length), note: 'Grouped by payment date', icon: 'payments' },
      { label: 'Average allocation', value: shortMoney(average), note: 'Exact paise arithmetic', icon: 'trend', tone: 'sky' },
      { label: 'Invoices with payments', value: String(paymentInvoices.size), note: 'Unique invoices touched', icon: 'users', tone: 'peach' },
    ];
    bars = methods.slice(0, 5).map(([method, value]) => moneyBar(method, value, max));
    insight = { eyebrow: 'Collection summary', title: payments.length ? String(payments.length) + ' allocations recorded' : 'No collections recorded', body: payments.length ? 'Payments are grouped by their recorded date. ' + paymentInvoices.size + ' invoices received at least one allocation during the selected period.' : 'There are no payment allocations with a payment date in the selected period.', footerLabel: 'Collected in period', footerValue: money(collectedTotal) };
  } else {
    const max = largestMoney([openTotal, overdueTotal, notYetDueTotal]);
    metrics = [
      { label: 'Open receivables', value: shortMoney(openTotal), note: String(receivableInvoices.length) + ' invoices with a balance', icon: 'payments', tone: 'peach' },
      { label: 'Overdue balance', value: shortMoney(overdueTotal), note: String(overdue.length) + ' past-due invoices', icon: 'clock', tone: 'sky' },
      { label: 'Not yet due', value: shortMoney(notYetDueTotal), note: String(notYetDue.length) + ' invoices due later', icon: 'trend', tone: 'mint' },
      { label: 'Billable invoices', value: String(receivableInvoices.length), note: 'Open balances in selected range', icon: 'users' },
    ];
    bars = [moneyBar('Past due', overdueTotal, max), moneyBar('Not yet due', notYetDueTotal, max), moneyBar('Total open', openTotal, max)];
    insight = { eyebrow: 'Receivables summary', title: overdue.length ? String(overdue.length) + ' overdue invoices' : 'No overdue balances', body: overdue.length ? String(overdue.length) + ' invoices are past due, with ' + money(overdueTotal) + ' outstanding. Review collection follow-up for the selected issue-date range.' : 'There are ' + money(openTotal) + ' in open balances for this issue-date range, and none are past due.', footerLabel: 'Open balance in period', footerValue: money(openTotal) };
  }

  const startLabel = rangeStart ? dateFmt(rangeStart) : '';
  const rangeDescription = rangeStart ? startLabel + ' – ' + dateFmt(today) : 'All available records';
  return <>
    <PageHeading eyebrow="Finance" title="Reports" description="Review recurring revenue, invoice activity, collections, and receivables." actions={<Button variant="outline" icon="download" onClick={() => window.print()}>Print report</Button>} />
    <div className="report-toolbar">
      <div className="tab-strip" role="tablist" aria-label="Report type">
        {(['Revenue', 'Invoicing', 'Collections', 'Receivables'] as const).map((item) => <button type="button" role="tab" id={'report-tab-' + item.toLowerCase()} aria-controls="report-results" aria-selected={report === item} className={report === item ? 'tab active-tab' : 'tab'} key={item} onClick={() => setReport(item)}>{item}</button>)}
      </div>
      <label className="select-control" htmlFor="report-period">Time period <select id="report-period" aria-label="Report time period" value={period} onChange={(event) => setPeriod(event.target.value as ReportPeriod)}><option>All time</option><option>This month</option><option>This quarter</option><option>This year</option></select><Icon name="chevron" size={15} /></label>
    </div>
    <p className="muted" role="status" aria-live="polite">Showing {rangeDescription}. Invoice reports use issue date; collections use payment date. Monthly recurring revenue is the current run rate.</p>
    <div id="report-results" role="tabpanel" aria-labelledby={'report-tab-' + report.toLowerCase()} tabIndex={0}>
      <div className="report-grid">{metrics.map((metric) => <MetricCard key={metric.label} {...metric} />)}</div>
      <div className="report-detail-grid">
        <Card>
          <div className="card-heading"><div><div className="eyebrow">{report} · {period}</div><h2>{report === 'Revenue' ? 'Revenue performance' : report === 'Invoicing' ? 'Invoice lifecycle' : report === 'Collections' ? 'Collections by method' : 'Outstanding balances'}</h2><p>{rangeDescription}</p></div></div>
          <div className="report-bars">{bars.length ? bars.map((bar) => <div className="report-bar-row" key={bar.label}><div><span>{bar.label}</span><strong>{bar.display}</strong></div><div className="progress-track"><span style={{ width: reportBarPercent(bar) }} /></div></div>) : <p className="muted">No payment methods have collections in this period.</p>}</div>
        </Card>
        <Card className="report-insight">
          <span className="insight-icon"><Icon name={report === 'Collections' ? 'card' : report === 'Invoicing' ? 'invoices' : report === 'Receivables' ? 'clock' : 'trend'} size={18} /></span>
          <div className="eyebrow">{insight.eyebrow}</div><h2>{insight.title}</h2><p>{insight.body}</p>
          <div className="insight-footer"><span>{insight.footerLabel}</span><strong>{insight.footerValue}</strong></div>
        </Card>
      </div>
    </div>
  </>;
}

export function DiscountsPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.discounts.filter((discount) => `${discount.name} ${discount.scope}`.toLowerCase().includes(actions.search.toLowerCase()));
  return <><PageHeading eyebrow="Pricing rules" title="Discounts" description="Apply admin-managed discounts automatically when a subscription qualifies." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('discount')}>Create discount</Button>} /><div className="compact-stats"><Card><span>Active rules</span><strong>{data.discounts.filter((d) => d.active).length}</strong></Card><Card><span>Total usage</span><strong>{data.discounts.reduce((sum, d) => sum + d.usage, 0)}</strong></Card><Card><span>Usage remaining</span><strong>{data.discounts.reduce((sum, d) => sum + Math.max(d.limit - d.usage, 0), 0)}</strong></Card></div><Card className="discount-list-card"><TableToolbar title="Discount rules" count={`${rows.length} rules`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Discount</th><th>Type</th><th>Scope</th><th>Minimum purchase</th><th>Usage</th><th>Valid through</th><th>Status</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{rows.map((discount) => <tr key={discount.id}><td><strong>{discount.name}</strong><small className="cell-subtitle">{discount.kind === 'Percentage' ? `${discount.value}% off` : `${money(discount.value)} off`}</small></td><td>{discount.kind}</td><td>{discount.scope}</td><td><CellMoney value={discount.minimum} /></td><td><div className="usage-cell"><strong>{discount.usage}</strong><span>of {discount.limit}</span><div className="usage-track"><i style={{ width: `${Math.min(100, discount.usage / discount.limit * 100)}%` }} /></div></div></td><td>{dateFmt(discount.endsAt)}</td><td><Status value={discount.active ? 'Active' : 'Paused'} /></td><td><button type="button" className="icon-button" aria-label={`More actions for ${discount.name}`}><Icon name="more" /></button></td></tr>)}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="plans" title="No discount rules" description="Create a fixed or percentage offer with clear usage limits." />}</Card></>;
}

export function TaxesPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  return <><PageHeading eyebrow="Business settings" title="Taxes" description="Tax is calculated after discounts and rounded per invoice line." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('tax')}>Add tax rule</Button>} /><Card className="tax-note"><span className="insight-icon"><Icon name="shield" size={18} /></span><div><strong>Calculation order is fixed</strong><p>Each line applies its eligible discount first, calculates tax on the reduced amount, then rounds tax half-up to the nearest paise.</p></div></Card><Card className="tax-list-card"><TableToolbar title="Tax rules" count={`${data.taxes.length} rules`} search="" /><TableWrap><thead><tr><th>Tax name</th><th>Region / treatment</th><th>Rate</th><th>Applied to</th><th>Status</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{data.taxes.map((tax) => <tr key={tax.id}><td><strong>{tax.name}</strong></td><td>{tax.region}</td><td><strong>{(tax.rateBps / 100).toFixed(2)}%</strong></td><td>Eligible invoice lines</td><td><Status value={tax.active ? 'Active' : 'Paused'} /></td><td><button type="button" className="icon-button" aria-label={`More actions for ${tax.name}`}><Icon name="more" /></button></td></tr>)}</tbody></TableWrap></Card></>;
}

export function StaffPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  const rows = data.staff.filter((staff) => `${staff.name} ${staff.email} ${staff.role}`.toLowerCase().includes(actions.search.toLowerCase()));
  return <><PageHeading eyebrow="Business settings" title="Team members" description="Add employees and keep each person’s business access scoped to this workspace." actions={<Button variant="primary" icon="plus" onClick={() => actions.create('staff')}>Add team member</Button>} /><Card className="admin-note"><span className="insight-icon"><Icon name="shield" size={18} /></span><div><strong>Only workspace admins can add staff.</strong><p>Each employee must first create and verify an ORCA account. Add their verified account email to give them immediate Staff access. This flow does not send invitation email. Customer portal access stays separate from employee access.</p></div></Card><Card className="staff-list-card"><TableToolbar title="Workspace access" count={`${rows.length} members`} search={actions.search} setSearch={actions.setSearch} /><TableWrap><thead><tr><th>Member</th><th>Role</th><th>Status</th><th>Account</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{rows.map((member) => <tr key={member.id}><td><div className="person-cell"><Avatar initials={member.initials} size="sm" /><span><strong>{member.name}</strong><small>{member.email}</small></span></div></td><td><Status value={member.role} /></td><td><Status value={member.status} /></td><td>{member.status === 'Active' ? 'Verified ORCA account' : 'Account needs attention'}</td><td>{member.role === 'Admin' ? <span className="muted">You</span> : <button type="button" className="icon-button" aria-label={`More actions for ${member.name}`}><Icon name="more" /></button>}</td></tr>)}</tbody></TableWrap>{rows.length === 0 && <EmptyState icon="users" title="No other team members" description="Add a verified ORCA account to give an employee access to this workspace." action={<Button variant="primary" icon="plus" onClick={() => actions.create('staff')}>Add team member</Button>} />}</Card></>;
}

export function ExportsPage({ data, actions }: { data: WorkspaceData; actions: PageActions }) {
  return <><PageHeading eyebrow="Data & integrations" title="Data exports" description="Prepare a portable archive of business records, invoice evidence, and audit history." actions={<Button variant="primary" icon="download" onClick={actions.exportData}>Request full export</Button>} /><div className="export-intro-grid"><Card className="export-intro"><span className="export-mark"><Icon name="download" size={20} /></span><div><h2>Your business data, ready to move</h2><p>Exports include related records, stored invoice calculations, payment allocations, and a manifest with counts and checksums.</p><div className="export-includes"><span>Customers</span><span>Subscriptions</span><span>Invoices</span><span>Payments</span><span>Audit history</span></div></div></Card><Card className="export-privacy"><Icon name="shield" size={18} /><strong>Private by default</strong><p>Only authorized workspace members can request and download business archives. Secrets and sessions are excluded.</p></Card></div><Card><TableToolbar title="Export history" count={`${data.exports.length} requests`} search="" /><TableWrap><thead><tr><th>Export</th><th>Requested</th><th>Records included</th><th>Status</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{data.exports.map((job) => <tr key={job.id}><td><div className="file-cell"><span className="file-icon"><Icon name="invoices" size={16} /></span><span><strong>{job.name}</strong><small>{job.id}</small></span></div></td><td>{dateFmt(job.createdAt)}</td><td>{job.counts ? Object.values(job.counts).reduce((a, b) => a + b, 0) : '—'}</td><td><Status value={job.status} /></td><td>{job.status === 'Ready' ? <Button variant="outline" size="sm" icon="download" onClick={() => actions.downloadExport(job.id)}>Download</Button> : <span className="muted">Preparing</span>}</td></tr>)}</tbody></TableWrap>{data.exports.length === 0 && <EmptyState icon="download" title="No exports yet" description="Request an export to package this business’s records and billing evidence." action={<Button variant="primary" onClick={actions.exportData}>Request an export</Button>} />}</Card></>;
}

export function SettingsPage({ actions }: { actions: PageActions }) {
  const [connections, setConnections] = useState<ManagedMCPConnection[]>([]);
  const [connectionState, setConnectionState] = useState<'loading' | 'ready' | 'forbidden' | 'error'>('loading');
  const [connectionMessage, setConnectionMessage] = useState('');
  const [copyMessage, setCopyMessage] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);
  const [revokingId, setRevokingId] = useState('');
  const origin = window.location.origin;
  const mcpEndpoint = `${origin}/mcp`;
  const resourceMetadata = `${origin}/.well-known/oauth-protected-resource/mcp`;
  const authorizationMetadata = `${origin}/.well-known/oauth-authorization-server`;

  useEffect(() => {
    let active = true;
    setConnectionState('loading');
    setConnectionMessage('');
    void apiRequest<Record<string, unknown>>('/mcp/grants').then((result) => {
      if (!active) return;
      const values = Array.isArray(result.items) ? result.items : [];
      setConnections(values.flatMap((value) => {
        if (!value || typeof value !== 'object') return [];
        const record = value as Record<string, unknown>;
        if (typeof record.id !== 'string' || typeof record.clientId !== 'string') return [];
        return [{
          id: record.id,
          clientId: record.clientId,
          scopes: Array.isArray(record.scopes) ? record.scopes.filter((scope): scope is string => typeof scope === 'string') : [],
          expiresAt: typeof record.expiresAt === 'string' ? record.expiresAt : '',
          revokedAt: typeof record.revokedAt === 'string' ? record.revokedAt : undefined,
        }];
      }));
      setConnectionState('ready');
    }).catch((error: unknown) => {
      if (!active) return;
      if (error instanceof ApiError && error.status === 403) {
        setConnectionState('forbidden');
        setConnectionMessage('Only a workspace admin can view or revoke MCP connections.');
      } else if (error instanceof ApiError && error.status === 401) {
        setConnectionState('error');
        setConnectionMessage('Your session expired. Sign in again to manage MCP connections.');
      } else {
        setConnectionState('error');
        setConnectionMessage(error instanceof Error ? error.message : 'Could not load MCP connections.');
      }
    });
    return () => { active = false; };
  }, [refreshKey]);

  const copyValue = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopyMessage(`${label} copied.`);
    } catch {
      setCopyMessage(`Copy is unavailable here. Select and copy: ${value}`);
    }
  };

  const revokeConnection = async (connection: ManagedMCPConnection) => {
    const accepted = window.confirm(`Revoke MCP access for ${connection.clientId}? The client will lose access to this ORCA workspace.`);
    if (!accepted) return;
    setRevokingId(connection.id);
    setConnectionMessage('');
    try {
      await apiRequest(`/mcp/grants/${encodeURIComponent(connection.id)}`, { method: 'DELETE' });
      setConnections((items) => items.map((item) => item.id === connection.id ? { ...item, revokedAt: new Date().toISOString() } : item));
      setConnectionMessage(`Access revoked for ${connection.clientId}.`);
    } catch (error) {
      setConnectionMessage(error instanceof Error ? error.message : 'Could not revoke this MCP connection.');
    } finally {
      setRevokingId('');
    }
  };

  const jumpToMCP = () => document.getElementById('external-mcp')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  return <><PageHeading eyebrow="Business workspace" title="Settings" description="Manage your organization, team access, pricing rules, and external connections." /><div className="settings-grid"><button className="settings-card" type="button" onClick={() => actions.navigate('settings')}><span className="settings-icon"><Icon name="globe" /></span><span><strong>Business profile</strong><small>Company name, region, and currency</small></span><Icon name="arrow" size={16} /></button><button className="settings-card" type="button" onClick={() => actions.navigate('taxes')}><span className="settings-icon"><Icon name="card" /></span><span><strong>Taxes</strong><small>Tax rules and calculation order</small></span><Icon name="arrow" size={16} /></button><button className="settings-card" type="button" onClick={() => actions.navigate('discounts')}><span className="settings-icon"><Icon name="plans" /></span><span><strong>Discounts</strong><small>Promotions, limits, and eligibility</small></span><Icon name="arrow" size={16} /></button><button className="settings-card" type="button" onClick={() => actions.navigate('staff')}><span className="settings-icon"><Icon name="users" /></span><span><strong>Team members</strong><small>Add employees with verified ORCA accounts</small></span><Icon name="arrow" size={16} /></button><button className="settings-card" type="button" onClick={() => actions.navigate('exports')}><span className="settings-icon"><Icon name="download" /></span><span><strong>Data exports</strong><small>Portable business records and invoice evidence</small></span><Icon name="arrow" size={16} /></button><button className="settings-card" type="button" onClick={jumpToMCP}><span className="settings-icon"><Icon name="plug" /></span><span><strong>External MCP</strong><small>Connect instructions, scopes, and revocation</small></span><Icon name="arrow" size={16} /></button></div><Card className="integration-card"><div><div className="eyebrow">Connections</div><h2>Provider status</h2><p>Payment, email, and AI providers can be configured for this workspace.</p></div><div className="integration-row"><span><i className="provider-mark stripe">S</i><span><strong>Stripe test mode</strong><small>Checkout and payment reconciliation</small></span></span><Badge tone="neutral">Configured by owner</Badge></div><div className="integration-row"><span><i className="provider-mark mail-provider"><Icon name="mail" size={16} /></i><span><strong>Email delivery</strong><small>Invoices and customer notifications</small></span></span><Badge tone="neutral">Configured by owner</Badge></div><div className="integration-row"><span><i className="provider-mark ai-provider"><Icon name="spark" size={17} /></i><span><strong>Groq Copilot</strong><small>Optional agent provider</small></span></span><Badge tone="neutral">Configured by owner</Badge></div><div className="settings-footnote">Provider health and credentials are managed by the workspace owner; ORCA never displays secret values.</div></Card><section id="external-mcp"><Card className="integration-card mcp-integration-card"><div><div className="eyebrow">External Model Context Protocol</div><h2>Connect an MCP client</h2><p>Use OAuth discovery with this Streamable HTTP endpoint. The client opens ORCA for sign-in and consent; no API token is copied into the client manually.</p></div><div className="mcp-endpoint-row"><code>{mcpEndpoint}</code><Button variant="outline" size="sm" icon="plug" onClick={() => void copyValue(mcpEndpoint, 'MCP endpoint')}>Copy endpoint</Button></div>{copyMessage && <p className="form-hint" role="status">{copyMessage}</p>}<div className="mcp-setup"><h3>How to connect</h3><ol><li>In an MCP client that supports OAuth discovery and Streamable HTTP, add a remote server using the endpoint above.</li><li>When the client starts OAuth, ORCA displays the requested access scopes. Sign in and approve only the access the client needs.</li><li>Return to the client to finish setup. Authorized grants appear in the connection list below.</li></ol></div><div className="mcp-discovery-links"><span>Resource metadata <a href={resourceMetadata} target="_blank" rel="noreferrer">{resourceMetadata}</a></span><span>Authorization metadata <a href={authorizationMetadata} target="_blank" rel="noreferrer">{authorizationMetadata}</a></span></div><div className="mcp-scope-list"><strong>Available scopes</strong><div><Badge tone="blue">records:read</Badge><Badge tone="blue">reports:read</Badge><Badge tone="blue">billing:simulate</Badge><Badge tone="amber">actions:propose</Badge></div><small>External clients can read, explain, simulate, and propose supported actions. Mutations still require review inside ORCA.</small></div><div className="mcp-connection-heading"><div><h3>Authorized connections</h3><p>Revoking a grant blocks future MCP requests for this workspace.</p></div><Button variant="outline" size="sm" onClick={() => setRefreshKey((key) => key + 1)} disabled={connectionState === 'loading'}>Refresh</Button></div>{connectionState === 'loading' && <p className="empty-inline" role="status">Loading authorized MCP connections…</p>}{connectionState === 'forbidden' && <p className="empty-inline" role="status">{connectionMessage}</p>}{connectionState === 'error' && <div className="empty-inline"><p role="alert">{connectionMessage}</p><Button variant="outline" size="sm" onClick={() => setRefreshKey((key) => key + 1)}>Try again</Button></div>}{connectionMessage && connectionState === 'ready' && <p className="form-hint" role="status">{connectionMessage}</p>}{connectionState === 'ready' && connections.length === 0 && <p className="empty-inline">No MCP clients have authorized this workspace yet.</p>}{connectionState === 'ready' && connections.length > 0 && <div className="table-scroll" role="group" tabIndex={0} aria-label="Authorized MCP connections"><table className="data-table"><thead><tr><th>Client ID</th><th>Scopes</th><th>Expires</th><th>Status</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{connections.map((connection) => { const expired = connection.expiresAt ? new Date(connection.expiresAt).getTime() <= Date.now() : true; const state = connection.revokedAt ? 'Revoked' : expired ? 'Expired' : 'Active'; return <tr key={connection.id}><td><code>{connection.clientId}</code></td><td>{connection.scopes.length ? connection.scopes.join(', ') : 'No scopes recorded'}</td><td>{connection.expiresAt ? dateFmt(connection.expiresAt) : '—'}</td><td><Badge tone={state === 'Active' ? 'green' : state === 'Expired' ? 'amber' : 'neutral'}>{state}</Badge></td><td>{state === 'Active' ? <Button variant="outline" size="sm" disabled={revokingId === connection.id} onClick={() => void revokeConnection(connection)}>{revokingId === connection.id ? 'Revoking…' : 'Revoke'}</Button> : <span className="muted">—</span>}</td></tr>; })}</tbody></table></div>}</Card></section></>;
}

type ManagedMCPConnection = { id: string; clientId: string; scopes: string[]; expiresAt: string; revokedAt?: string };

export function CopilotPage({ data, messages, onSend, actions, busy = false, canRun = false, canReview = false }: {
  data: WorkspaceData; messages: { id: string; role: 'user' | 'assistant'; text: string; time: string }[];
  onSend: (prompt: string) => void; actions: PageActions; busy?: boolean; canRun?: boolean; canReview?: boolean;
}) {
  const [prompt, setPrompt] = useState('');
  const pending = data.approvals.filter((approval) => approval.status === 'Needs review');
  const resolved = data.approvals.filter((approval) => approval.status !== 'Needs review').slice(0, 4);
  const visibleApprovals = canReview ? pending : [];
  const submit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = prompt.trim();
    if (!value || busy || !canRun) return;
    onSend(value);
    setPrompt('');
  };
  return <>
    <PageHeading eyebrow="ORCA Copilot" title="Your billing assistant" description="Plan and complete business workflows with clear progress and approval at the right moment." actions={<Badge tone="purple">Human review enabled</Badge>} />
    <div className="copilot-grid">
      <Card className="chat-card">
        <div className="chat-header">
          <span className="copilot-avatar"><Icon name="spark" size={19} /></span>
          <span><strong>ORCA Copilot</strong><small>{canRun ? 'Connected to the signed-in workspace' : 'AI service is not configured or is unavailable'}</small></span>
          <Badge tone={canRun ? 'green' : 'neutral'}>{canRun ? 'Available' : 'Unavailable'}</Badge>
        </div>
        <div className="chat-messages" aria-live="polite">
          {messages.map((message) => <div key={message.id} className={`chat-message ${message.role === 'user' ? 'user-message' : 'assistant-message'}`}><div className="chat-bubble">{message.text}</div><small>{message.time}</small></div>)}
          {busy && <div className="assistant-message"><div className="typing-indicator"><i /><i /><i /></div><small>Working through your request…</small></div>}
        </div>
        <div className="prompt-suggestions">
          <span>Try asking</span>
          <button type="button" onClick={() => setPrompt('Find open invoices for a customer')}>Find open invoices</button>
          <button type="button" onClick={() => setPrompt('Summarize workspace billing and invoice integrity')}>Summarize billing</button>
          <button type="button" onClick={() => setPrompt('Find an invoice and explain how its total was calculated')}>Explain an invoice</button>
        </div>
        <form className="chat-input-row" onSubmit={submit}>
          <label className="sr-only" htmlFor="copilot-prompt">Ask ORCA Copilot</label>
          <input id="copilot-prompt" value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder={canRun ? 'Ask ORCA to help with your billing…' : 'Copilot is unavailable right now'} disabled={!canRun || busy} />
          <Button variant="primary" type="submit" ariaLabel="Send prompt" icon="arrow" disabled={!canRun || !prompt.trim() || busy} />
        </form>
        <p className="chat-disclaimer">Copilot reads records, explains invoices, simulates upcoming bills, and prepares action proposals. Approved proposals execute only after the server rechecks access and record versions. Prompts are sent only when the configured AI service is available.</p>
      </Card>
      <div className="copilot-side">
        <Card>
          <div className="card-heading">
            <div><div className="eyebrow">Task approvals</div><h2>Review before action</h2><p>Each proposal shows its exact target and inputs. Approval executes the action after server revalidation.</p></div>
            <span className="approval-count">{visibleApprovals.length}</span>
          </div>
          {visibleApprovals.length === 0
            ? <EmptyState icon="shield" title={canReview ? 'All caught up' : 'Review access required'} description={canReview ? 'New proposals will appear here for review.' : 'A workspace admin or staff member can review action proposals.'} />
            : <div className="approval-list">{visibleApprovals.map((approval) => {
              const invoice = data.invoices.find((item) => item.id === approval.target || item.number === approval.target);
              const subscription = data.subscriptions.find((item) => item.id === approval.target || item.number === approval.target);
              return <article className="approval-card" key={approval.id}>
                <div className="approval-top"><Badge tone="amber">{approval.status}</Badge><span>{approval.createdAt}</span></div>
                <h3>{approval.title}</h3><p>{approval.detail}</p>
                <div className="approval-target"><span>Action</span><strong>{approval.action}</strong><span>Target</span>
                  <button type="button" className="link-button" onClick={() => { if (invoice) actions.navigate('invoice', invoice.id); else if (subscription) actions.navigate('subscriptions'); }}>{approval.target}</button>
                </div>
                <div className="approval-actions">
                  <Button variant="outline" size="sm" disabled={!canReview} onClick={() => actions.decideApproval(approval, 'reject')}>Decline</Button>
                  <Button variant="primary" size="sm" icon="check" disabled={!canReview} onClick={() => actions.decideApproval(approval, 'approve')}>Approve and execute</Button>
                </div>
              </article>;
            })}</div>}
          {canReview && resolved.length > 0 && <div className="resolved-approvals">
            <h3>Recent decisions</h3>
            {resolved.map((approval) => {
              const result = approval.executionResult ?? {};
              const outcome = result.paymentId && typeof result.amount === 'string'
                ? `Payment ${money(result.amount)} recorded${typeof result.outstanding === 'string' ? `; ${money(result.outstanding)} remains` : ''}.`
                : result.queued === true ? 'Email request queued.'
                  : typeof result.status === 'string' ? `Completed with status ${result.status}.`
                    : approval.status === 'Declined' ? 'The proposal was declined; no action was taken.' : 'Proposal approved.';
              return <article key={approval.id}><div><Badge tone={approval.status === 'Approved' ? 'green' : 'neutral'}>{approval.status}</Badge><strong>{approval.action}</strong></div><span>{outcome}</span></article>;
            })}
          </div>}
        </Card>
        <Card className="agent-coverage">
          <div className="eyebrow">Current Copilot tools</div>
          <div className="coverage-list">
            <span><Icon name="customers" size={15} />Search customers, plans, and subscriptions</span>
            <span><Icon name="invoices" size={15} />Explain stored invoice calculations</span>
            <span><Icon name="subscriptions" size={15} />Preview the next invoice</span>
            <span><Icon name="shield" size={15} />Prepare proposals for billing actions</span>
          </div>
          <button className="text-link" type="button" onClick={() => actions.navigate('invoices')}>Explore workspace records <Icon name="arrow" size={15} /></button>
        </Card>
      </div>
    </div>
  </>;
}

export function PortalPage({ data, actions, isCustomer, canPayOnline }: { data: WorkspaceData; actions: PageActions; isCustomer: boolean; canPayOnline: boolean }) {
  if (!isCustomer) {
    return <><PageHeading eyebrow="Customer portal" title="Customer access required" description="Sign in with a customer portal account to view subscriptions, invoices, and payments." /><EmptyState icon="shield" title="This account is not linked to a customer" description="Ask the business to create a customer record with this account’s email, then use customer portal access." /></>;
  }
  const customer = data.customers[0];
  if (!customer) {
    return <><PageHeading eyebrow="Customer portal" title="Customer record unavailable" description="This account does not have a customer record linked in the current business." /><EmptyState icon="customers" title="No linked customer record" description="Contact the business workspace admin to verify your portal access." /></>;
  }
  const subscriptions = data.subscriptions.filter((sub) => sub.customerId === customer?.id && ['Active', 'Paused'].includes(sub.status));
  const invoices = data.invoices.filter((invoice) => invoice.customerId === customer?.id);
  return <>
    <PageHeading eyebrow="Customer portal" title={`Welcome, ${customer.name}`} description="A focused view of your own subscriptions, invoices, and payments." actions={<Badge tone="blue">Your account</Badge>} />
    <div className="portal-banner"><span className="portal-brand"><Icon name="overview" size={17} />{customer.company || 'Customer portal'}</span><span>{customer.email}</span></div>
    <div className="compact-stats">
      <Card><span>Active subscriptions</span><strong>{subscriptions.filter((sub) => sub.status === 'Active').length}</strong></Card>
      <Card><span>Open balance</span><strong className="small-stat-money">{money(invoices.filter((invoice) => invoice.status === 'Confirmed').reduce((sum, invoice) => addMoney(sum, outstanding(invoice)), '0.00' as Money))}</strong></Card>
      <Card><span>Payments</span><strong>{data.payments.filter((payment) => payment.customerId === customer?.id).length}</strong></Card>
    </div>
    <Card className="portal-subscriptions">
      <TableToolbar title="Your subscriptions" count={`${subscriptions.length} agreements`} search="" />
      <TableWrap><thead><tr><th>Subscription</th><th>Plan</th><th>Seats</th><th>Next renewal</th><th>Status</th><th className="align-right">Amount</th><th><span className="sr-only">Actions</span></th></tr></thead>
        <tbody>{subscriptions.map((sub) => { const plan = data.plans.find((item) => item.id === sub.planId); return <tr key={sub.id}><td><strong>{sub.number}</strong></td><td>{plan?.name}</td><td>{sub.seats}</td><td>{dateFmt(sub.renewsAt)}</td><td><Status value={sub.status} /></td><td className="align-right"><CellMoney value={sub.amount} /></td><td>{sub.status === 'Active' && plan?.pauseAllowed ? <button type="button" className="small-action" onClick={() => actions.transitionSubscription(sub.id, 'Pause')}>Request pause</button> : <span className="muted">—</span>}</td></tr>; })}</tbody>
      </TableWrap>
    </Card>
    <Card className="portal-invoices">
      <TableToolbar title="Your invoices" count={`${invoices.length} records`} search="" />
      <TableWrap><thead><tr><th>Invoice</th><th>Period</th><th>Due</th><th>Status</th><th className="align-right">Balance</th><th>Actions</th></tr></thead>
        <tbody>{invoices.map((invoice) => {
          const balance = invoice.status === 'Confirmed' ? outstanding(invoice) : '0.00' as Money;
          const canPay = canPayOnline && invoice.status === 'Confirmed' && toPaise(balance) > 0n;
          return <tr key={invoice.id}>
            <td><button className="link-button" type="button" onClick={() => actions.navigate('invoice', invoice.id)}>{invoice.number}</button></td><td>{invoice.period}</td><td>{dateFmt(invoice.dueDate)}</td><td><Status value={isOverdue(invoice) ? 'Overdue' : invoice.status} /></td>
            <td className="align-right"><CellMoney value={balance} /></td>
            <td><div className="portal-invoice-actions">{canPay && <PayOnlineAction size="sm" invoice={invoice} onPay={actions.payOnline} />}</div></td>
          </tr>;
        })}</tbody>
      </TableWrap>
    </Card>
  </>;
}
