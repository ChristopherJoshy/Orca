import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiRequest } from './api';
import { Icon, type IconName } from './icons';
import { addMoney, fromPaise, money, multiplyMoney, outstanding, toPaise, type Approval, type Invoice, type Plan, type Product, type TaxRule, type WorkspaceData } from './model';
import type { PageActions, PageKey } from './pages';
import { CatalogPage, CopilotPage, CustomersPage, DiscountsPage, ExportsPage, InvoiceTruthPage, InvoicesPage, OverviewPage, PaymentsPage, PlansPage, PortalPage, QuotationsPage, ReportsPage, SettingsPage, StaffPage, SubscriptionsPage, TaxesPage } from './pages';
import { Avatar, Badge, Button, Dialog, Form, FormActions, IconButton, SelectField, TextField, ToggleField } from './ui';
import { useWorkspace } from './useWorkspace';

const pagePaths: Record<PageKey, string> = {
  overview: '/', customers: '/customers', catalog: '/catalog', plans: '/plans', quotations: '/quotations',
  subscriptions: '/subscriptions', invoices: '/invoices', invoice: '/invoices', payments: '/payments',
  reports: '/reports', settings: '/settings', discounts: '/settings/discounts', taxes: '/settings/taxes',
  staff: '/settings/staff', exports: '/settings/exports', copilot: '/copilot', portal: '/portal',
};
const pageTitles: Record<PageKey, string> = {
  overview: 'Overview', customers: 'Customers', catalog: 'Catalog', plans: 'Plans', quotations: 'Quotations',
  subscriptions: 'Subscriptions', invoices: 'Invoices', invoice: 'Invoice Truth', payments: 'Payments', reports: 'Reports',
  settings: 'Settings', discounts: 'Discounts', taxes: 'Taxes', staff: 'Team members', exports: 'Data exports', copilot: 'Copilot', portal: 'Customer portal',
};
const primaryNav: { page: PageKey; icon: IconName }[] = [
  { page: 'overview', icon: 'overview' }, { page: 'customers', icon: 'customers' }, { page: 'catalog', icon: 'catalog' },
  { page: 'plans', icon: 'plans' }, { page: 'quotations', icon: 'quotations' }, { page: 'subscriptions', icon: 'subscriptions' },
  { page: 'invoices', icon: 'invoices' }, { page: 'payments', icon: 'payments' }, { page: 'reports', icon: 'reports' },
];

type Route = { page: PageKey; id?: string };
type ChatMessage = { id: string; role: 'user' | 'assistant'; text: string; time: string };
type CreateKind = 'customer' | 'product' | 'plan' | 'quotation' | 'quoteTemplate' | 'subscription' | 'discount' | 'staff' | 'tax';
type CatalogEditTarget = { kind: 'product'; record: Product } | { kind: 'plan'; record: Plan };
type CatalogArchiveTarget = { kind: 'product'; record: Product } | { kind: 'plan'; record: Plan };

function routeFromLocation(): Route {
  const path = window.location.pathname.replace(/\/$/, '') || '/';
  if (path.startsWith('/invoices/')) return { page: 'invoice', id: decodeURIComponent(path.slice('/invoices/'.length)) };
  const page = (Object.keys(pagePaths) as PageKey[]).find((key) => pagePaths[key] === path);
  if (page) return { page };
  if (path === '/settings/discounts') return { page: 'discounts' };
  if (path === '/settings/taxes') return { page: 'taxes' };
  if (path === '/settings/staff') return { page: 'staff' };
  if (path === '/settings/exports') return { page: 'exports' };
  return { page: 'overview' };
}

function formatDate(value: Date) {
  return new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', year: 'numeric' }).format(value);
}

function responseId(response: unknown, fallback: string) {
  if (!response || typeof response !== 'object') return fallback;
  const outer = response as Record<string, unknown>;
  const nested = [outer.product, outer.plan, outer.customer, outer.subscription, outer.quotation, outer.discount, outer.staff, outer.tax, outer.invoice, outer.data]
    .find((value) => value && typeof value === 'object') as Record<string, unknown> | undefined;
  return String(outer.id ?? nested?.id ?? fallback);
}

function checkoutDestination(value: unknown): string {
  const invalidResponse = () => new Error('Could not start a secure checkout. Try again or contact the business.');
  if (!value || typeof value !== 'object') throw invalidResponse();
  const response = value as Record<string, unknown>;
  const amount = response.amount;
  if (typeof response.sessionId !== 'string' || !response.sessionId.trim() || typeof amount !== 'string' || !/^(?:0|[1-9]\d*)\.\d{2}$/.test(amount) || toPaise(amount) <= 0n || response.currency !== 'INR' || typeof response.url !== 'string') {
    throw invalidResponse();
  }
  let destination: URL;
  try { destination = new URL(response.url); } catch { throw invalidResponse(); }
  if (destination.origin !== 'https://checkout.stripe.com' || destination.username || destination.password) throw invalidResponse();
  return destination.href;
}

function onlinePaymentFailure(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) return 'Session expired. Sign in again and retry.';
    if (error.status === 403) return 'This invoice cannot be paid online from this account.';
    if (error.status === 409) return 'The invoice balance changed. Refresh and retry.';
    if (error.status === 503) return 'Online payments are not configured for this business. Contact them to arrange payment.';
    return 'Could not start online payment. Try again.';
  }
  if (error instanceof TypeError) return 'Could not connect to online payments. Check your connection and try again.';
  return error instanceof Error ? error.message : 'Could not start online payment. Try again.';
}

function initials(name: string) { return name.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase(); }
function percentageBps(value: string) {
  if (!/^\d+(?:\.\d{1,2})?$/.test(value)) throw new Error('Tax rate must be a number with up to two decimal places.');
  const [whole, fraction = ''] = value.split('.');
  const bps = Number(BigInt(whole) * 100n + BigInt(fraction.padEnd(2, '0')));
  if (!Number.isSafeInteger(bps) || bps > 10000) throw new Error('Tax rate must be between 0 and 100 percent.');
  return bps;
}
function percentageString(bps: number) { return `${Math.floor(bps / 100)}.${String(bps % 100).padStart(2, '0')}`; }

function CreateRecordDialog({ kind, data, onClose, onSave }: {
  kind: CreateKind;
  data: WorkspaceData; onClose: () => void; onSave: (values: Record<string, string | boolean>) => Promise<boolean>;
}) {
  const [values, setValues] = useState<Record<string, string | boolean>>(() => ({
    name: '', email: '', company: '', phone: '', type: 'Software', price: '', cost: '', notes: '', variantAttribute: '', variantValue: '', variantAdjustment: '0.00',
    taxRate: percentageString(data.taxes.find((tax) => tax.active)?.rateBps ?? 0),
    cadence: 'Monthly', minimumQuantity: '1', availableFrom: new Date().toISOString().slice(0, 10), availableUntil: '',
    pauseAllowed: true, closeAllowed: true, autoClose: false, renewAllowed: true,
    customerId: data.customers.at(-1)?.id ?? '', planId: data.plans[0]?.id ?? '', quantity: String(data.plans[0]?.minimumQuantity ?? 1), validDays: '14', title: '', templateId: '',
    templatePlanId: data.plans[0]?.id ?? '', templateProductId: '', templateProductVariant: '', templateQuantity: String(data.plans[0]?.minimumQuantity ?? 1),
    paymentTerms: '30', discountKind: 'Percentage', value: '10', scope: 'Subscriptions', minimum: '0.00', usageLimit: '25', endsAt: '',
    startsAt: '',
    productId: '', productVariant: '', productQuantity: '1',
    region: 'India · Standard rated', rate: '18', role: 'Staff',
  }));
  const [busy, setBusy] = useState(false);
  const set = (key: string, value: string | boolean) => setValues((current) => ({ ...current, [key]: value }));
  const v = (key: string) => String(values[key] ?? '');
  const title: Record<typeof kind, string> = { customer: 'Add a customer', product: 'Add a product', plan: 'Create a plan', quotation: 'Prepare a quotation', quoteTemplate: 'Create quotation template', subscription: 'New subscription', discount: 'Create discount', staff: 'Add verified ORCA account', tax: 'Add tax rule' };
  const subtitle: Record<typeof kind, string> = { customer: 'Add a customer record to your workspace.', product: 'Set a base price and optional additive variant.', plan: 'Define billing frequency and lifecycle permissions.', quotation: 'Choose a customer and offer, then set validity.', quoteTemplate: 'Save a plan and optional product line to reuse in customer offers.', subscription: 'Create a draft subscription for a customer.', discount: 'Set eligibility, scope, and a usage limit.', staff: 'The teammate must create an ORCA account and verify this email before you can add them.', tax: 'Configure a tax rate for eligible invoice lines.' };
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true);
    const saved = await onSave(values);
    setBusy(false);
    if (saved) onClose();
  };
  const text = (label: string, key: string, type = 'text', placeholder = '', required = true, min?: string, step?: string) => <TextField label={label} name={key} type={type} value={v(key)} onChange={(value) => set(key, value)} placeholder={placeholder} required={required} min={min} step={step} />;
  const select = (label: string, key: string, options: { label: string; value: string }[]) => <SelectField label={label} name={key} value={v(key)} onChange={(value) => set(key, value)} options={options} required />;
  const customerOptions = data.customers.map((customer) => ({ label: `${customer.company} · ${customer.name}`, value: customer.id }));
  const planOptions = data.plans.map((plan) => ({ label: `${plan.name} · ${money(plan.price)}`, value: plan.id }));
  const addOnOptions = data.products.map((product) => ({ label: `${product.name} · ${money(product.price)}`, value: product.id }));
  const selectedAddOn = data.products.find((product) => product.id === v('productId'));
  const variantOptions = selectedAddOn?.variants.map((variant) => ({ label: `${variant.name} · ${money(variant.adjustment)} adjustment`, value: variant.value })) ?? [];
  const selectedTemplateProduct = data.products.find((product) => product.id === v('templateProductId'));
  const templateVariantOptions = selectedTemplateProduct?.variants.map((variant) => ({ label: `${variant.name} · ${money(variant.adjustment)} adjustment`, value: variant.value })) ?? [];
  const templateOptions = data.quoteTemplates.map((template) => ({ label: `${template.name} · ${template.validDays} days`, value: template.id }));
  const selectedPlanMinimum = Math.max(1, data.plans.find((plan) => plan.id === v('planId'))?.minimumQuantity ?? 1);
  const selectedTemplatePlanMinimum = Math.max(1, data.plans.find((plan) => plan.id === v('templatePlanId'))?.minimumQuantity ?? 1);
  return <Dialog title={title[kind]} description={subtitle[kind]} onClose={onClose}>
    <Form onSubmit={submit}>
      {kind === 'customer' && <div className="form-grid">{text('Contact name', 'name', 'text', 'e.g. Leena Shah')}{text('Email address', 'email', 'email', 'name@company.example')}{text('Company', 'company', 'text', 'Company name')}{text('Phone', 'phone', 'tel', '+91 90000 00000', false)}</div>}
      {kind === 'product' && <div className="form-grid">{text('Product name', 'name', 'text', 'e.g. Orca Business')}{select('Product type', 'type', ['Software', 'Add-on', 'Service', 'Physical product'].map((value) => ({ label: value, value })))}{text('Sales price · INR', 'price', 'number', '2499.00', true, '0', '0.01')}{text('Cost price · INR', 'cost', 'number', '620.00', true, '0', '0.01')}{text('Tax rate (%)', 'taxRate', 'number', '18.00', true, '0', '0.01')}{text('Notes', 'notes', 'text', 'Optional product notes', false)}<div className="form-section-label">Optional variant</div>{text('Variant attribute', 'variantAttribute', 'text', 'e.g. Brand', false)}{text('Variant value', 'variantValue', 'text', 'e.g. Odoo', false)}{text('Price adjustment · INR', 'variantAdjustment', 'number', '560.00', false, '0', '0.01')}</div>}
      {kind === 'plan' && <><div className="form-grid">{text('Plan name', 'name', 'text', 'Business monthly')}{text('Plan price · INR', 'price', 'number', '2499.00', true, '0', '0.01')}{select('Billing interval', 'cadence', ['Daily', 'Weekly', 'Monthly', 'Yearly'].map((value) => ({ label: value, value })))}{text('Minimum quantity', 'minimumQuantity', 'number', '1', true, '1', '1')}{text('Tax rate (%)', 'taxRate', 'number', '18.00', true, '0', '0.01')}{text('Available from', 'availableFrom', 'date')}{text('Available until', 'availableUntil', 'date', '', false)}</div><div className="form-toggle-stack"><ToggleField label="Allow pause" checked={Boolean(values.pauseAllowed)} onChange={(value) => set('pauseAllowed', value)} /><ToggleField label="Allow early close" checked={Boolean(values.closeAllowed)} onChange={(value) => set('closeAllowed', value)} /><ToggleField label="Auto-close at term end" description="Automatically close the subscription when its term ends." checked={Boolean(values.autoClose)} onChange={(value) => set('autoClose', value)} /><ToggleField label="Allow renewal" checked={Boolean(values.renewAllowed)} onChange={(value) => set('renewAllowed', value)} /></div></>}
      {kind === 'quotation' && <div className="form-grid">{text('Quotation title', 'title', 'text', 'e.g. Core monthly offer')}{select('Customer', 'customerId', customerOptions)}<SelectField label="Start from a saved template (optional)" name="templateId" value={v('templateId')} onChange={(value) => { set('templateId', value); set('productId', ''); set('productVariant', ''); }} options={[{ label: 'Choose a plan below', value: '' }, ...templateOptions]} />{!v('templateId') && <>{select('Plan', 'planId', planOptions)}{text(`Quantity · minimum ${selectedPlanMinimum}`, 'quantity', 'number', String(selectedPlanMinimum), true, String(selectedPlanMinimum), '1')}{text('Valid for (days)', 'validDays', 'number', '14', true, '1', '1')}<SelectField label="Optional product or add-on" name="productId" value={v('productId')} onChange={(value) => { set('productId', value); set('productVariant', ''); }} options={[{ label: 'No additional product', value: '' }, ...addOnOptions]} />{selectedAddOn && <>{variantOptions.length > 0 && <SelectField label="Product variant" name="productVariant" value={v('productVariant')} onChange={(value) => set('productVariant', value)} options={[{ label: 'Base product price', value: '' }, ...variantOptions]} />}{text('Add-on quantity', 'productQuantity', 'number', '1', true, '1', '1')}</>}</>}</div>}
      {kind === 'quoteTemplate' && <div className="form-grid">{text('Template name', 'name', 'text', 'e.g. Business monthly offer')}{select('Billing plan', 'templatePlanId', planOptions)}<SelectField label="Additional product (optional)" name="templateProductId" value={v('templateProductId')} onChange={(value) => { set('templateProductId', value); set('templateProductVariant', ''); }} options={[{ label: 'No additional product', value: '' }, ...data.products.map((product) => ({ label: `${product.name} · ${money(product.price)}`, value: product.id }))]} />{selectedTemplateProduct && templateVariantOptions.length > 0 && <SelectField label="Product variant" name="templateProductVariant" value={v('templateProductVariant')} onChange={(value) => set('templateProductVariant', value)} options={[{ label: 'Base product price', value: '' }, ...templateVariantOptions]} />}{text(`Plan quantity · minimum ${selectedTemplatePlanMinimum}`, 'templateQuantity', 'number', String(selectedTemplatePlanMinimum), true, String(selectedTemplatePlanMinimum), '1')}{text('Offer validity (days)', 'validDays', 'number', '14', true, '1', '1')}</div>}
      {kind === 'subscription' && <div className="form-grid">{select('Customer', 'customerId', customerOptions)}{select('Plan', 'planId', planOptions)}{text(`Quantity · minimum ${selectedPlanMinimum}`, 'quantity', 'number', String(selectedPlanMinimum), true, String(selectedPlanMinimum), '1')}{select('Payment terms', 'paymentTerms', [{ label: 'Due on receipt', value: '0' }, { label: 'Net 15', value: '15' }, { label: 'Net 30', value: '30' }])}{text('Start date', 'availableFrom', 'date')}<SelectField label="Optional product or add-on" name="productId" value={v('productId')} onChange={(value) => { set('productId', value); set('productVariant', ''); }} options={[{ label: 'No additional product', value: '' }, ...addOnOptions]} />{selectedAddOn && <>{variantOptions.length > 0 && <SelectField label="Product variant" name="productVariant" value={v('productVariant')} onChange={(value) => set('productVariant', value)} options={[{ label: 'Base product price', value: '' }, ...variantOptions]} />}{text('Add-on quantity', 'productQuantity', 'number', '1', true, '1', '1')}</>}</div>}
      {kind === 'discount' && <div className="form-grid">{text('Discount name', 'name', 'text', 'Annual commitment')}{select('Discount type', 'discountKind', [{ label: 'Percentage', value: 'Percentage' }, { label: 'Fixed amount', value: 'Fixed' }])}{text(values.discountKind === 'Fixed' ? 'Amount · INR' : 'Percentage off', 'value', 'number', values.discountKind === 'Fixed' ? '1500.00' : '10', true, '0', '0.01')}{select('Applies to', 'scope', [{ label: 'Entire subscription', value: 'Subscriptions' }, ...data.products.map((product) => ({ label: product.name, value: product.id }))])}{text('Minimum purchase · INR', 'minimum', 'number', '0.00', true, '0', '0.01')}{text('Minimum quantity', 'minimumQuantity', 'number', '1', true, '1', '1')}{text('Usage limit', 'usageLimit', 'number', '25', true, '1', '1')}{text('Starts on', 'startsAt', 'date', '', false)}{text('Ends on', 'endsAt', 'date', '', false)}</div>}
      {kind === 'staff' && <div className="form-grid">{text('Teammate name', 'name', 'text', 'Sam Patel')}{text('Email on the verified ORCA account', 'email', 'email', 'teammate@company.com')}<p className="form-hint">Ask your teammate to create an ORCA account and verify this exact email first. Adding them grants staff access to this workspace.</p></div>}
      {kind === 'tax' && <div className="form-grid">{text('Tax name', 'name', 'text', 'GST')}{text('Region or treatment', 'region', 'text', 'India · Standard rated')}{text('Rate (%)', 'rate', 'number', '18', true, '0', '0.01')}</div>}
      <FormActions onCancel={onClose} submit={kind === 'staff' ? 'Add staff member' : kind === 'quotation' ? 'Prepare quotation' : kind === 'subscription' ? 'Create draft' : kind === 'quoteTemplate' ? 'Save template' : 'Save'} disabled={busy} />
    </Form>
  </Dialog>;
}

function PaymentDialog({ invoice, customer, onClose, onSave }: { invoice: Invoice; customer: string; onClose: () => void; onSave: (values: { amount: string; method: string; date: string; note: string }) => Promise<boolean> }) {
  const [amount, setAmount] = useState(outstanding(invoice));
  const [method, setMethod] = useState('Bank transfer');
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10));
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    try {
      if (toPaise(amount) <= 0n || toPaise(amount) > toPaise(outstanding(invoice))) throw new Error(`Amount must be between ₹0.01 and ${money(outstanding(invoice))}.`);
      setError(''); setBusy(true);
      const saved = await onSave({ amount: fromPaise(toPaise(amount)), method, date, note });
      setBusy(false); if (saved) onClose();
    } catch (e) { setError(e instanceof Error ? e.message : 'Enter a valid amount.'); setBusy(false); }
  };
  return <Dialog title="Record payment" description={`Allocate a payment to ${invoice.number} for ${customer}.`} onClose={onClose}>
    <Form onSubmit={submit}><div className="payment-target"><span>Outstanding balance</span><strong>{money(outstanding(invoice))}</strong></div><div className="form-grid">{<TextField label="Payment amount · INR" name="payment-amount" type="number" value={amount} onChange={setAmount} required min="0.01" step="0.01" error={error} />}<SelectField label="Payment method" name="payment-method" value={method} onChange={setMethod} options={['Bank transfer', 'UPI', 'Card', 'Cash', 'Other'].map((value) => ({ label: value, value }))} /><TextField label="Received on" name="payment-date" type="date" value={date} onChange={setDate} required /><TextField label="Reference note" name="payment-note" value={note} onChange={setNote} placeholder="Optional note" /></div><p className="form-hint">This allocation updates the invoice balance and ledger after the workspace approves the request.</p><FormActions onCancel={onClose} submit="Review payment" disabled={busy} /></Form>
  </Dialog>;
}

type AuthMode = 'login' | 'signup' | 'customer' | 'forgot' | 'reset';

function AuthScreen({ apiState, connectionError, onRetry, onAuthenticated }: { apiState: 'checking' | 'online' | 'offline'; connectionError: string; onRetry: () => void; onAuthenticated: () => void }) {
  const [mode, setMode] = useState<AuthMode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [business, setBusiness] = useState('');
  const [resetToken, setResetToken] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const queryTokenHandled = useRef(false);

  useEffect(() => {
    if (queryTokenHandled.current) return;
    if (apiState === 'checking') return;
    const params = new URLSearchParams(window.location.search);
    const verifyToken = params.get('verify');
    const passwordToken = params.get('reset');
    if (!verifyToken && !passwordToken) return;
    if (apiState !== 'online') {
      setError('ORCA cannot reach the API yet. Retry the connection to continue with this secure link.');
      return;
    }
    queryTokenHandled.current = true;
    window.history.replaceState({}, '', window.location.pathname || '/');
    if (passwordToken) {
      setResetToken(passwordToken);
      setMode('reset');
      setNotice('Choose a new password for your ORCA account.');
      return;
    }
    setBusy(true);
    void apiRequest('/auth/verify-email', { method: 'POST', body: JSON.stringify({ token: verifyToken }) })
      .then(() => onAuthenticated())
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'This verification link is invalid or expired.'))
      .finally(() => setBusy(false));
  }, [apiState, onAuthenticated]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(''); setNotice(''); setBusy(true);
    try {
      if (mode === 'forgot') {
        await apiRequest('/auth/password-reset', { method: 'POST', body: JSON.stringify({ email }) });
        setNotice('If that account can receive email, password reset instructions are on the way.');
      } else if (mode === 'reset') {
        await apiRequest('/auth/password-reset/complete', { method: 'POST', body: JSON.stringify({ token: resetToken, password }) });
        setMode('login'); setPassword(''); setNotice('Password updated. Sign in with your new password.');
      } else {
        const requestBody = mode === 'login' ? { email, password } : mode === 'customer' ? { email, password, businessId: business.trim() } : { email, password, businessName: business.trim() };
        const result = await apiRequest<Record<string, unknown>>(mode === 'login' ? '/auth/login' : '/auth/signup', { method: 'POST', body: JSON.stringify(requestBody) });
        if (typeof result.csrfToken === 'string') { onAuthenticated(); return; }
        if (mode === 'customer') {
          setNotice('If the business has a matching customer record, ORCA will send an email verification link.');
        } else {
          setNotice(result.emailSent === true
            ? 'Account created. Open the verification email to finish signing in.'
            : 'Account created. Email verification must be configured before this account can sign in.');
        }
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'ORCA could not complete this request. Please try again.');
    } finally {
      setBusy(false);
    }
  };

  const isLoading = apiState === 'checking';
  const title = mode === 'login' ? 'Welcome back.' : mode === 'signup' ? 'Start with a clear view.' : mode === 'customer' ? 'Access your account.' : mode === 'forgot' ? 'Reset your password.' : 'Choose a new password.';
  const description = mode === 'login' ? 'Sign in to your billing workspace.' : mode === 'signup' ? 'Create a business workspace with your first admin account.' : mode === 'customer' ? 'Link a verified email to your customer record.' : mode === 'forgot' ? 'We’ll send a secure reset link if the account can receive email.' : 'Use a strong password to secure your account.';
  const inputDisabled = busy || isLoading || apiState === 'offline' || Boolean(connectionError);

  return <main className="auth-page">
    <section className="auth-story" aria-label="About ORCA">
      <div className="auth-story-orbit orbit-one" /><div className="auth-story-orbit orbit-two" />
      <div className="auth-story-inner">
        <div className="auth-brand"><span className="auth-brand-mark"><Icon name="overview" size={21} /></span><span className="auth-brand-name">orca</span><span className="auth-brand-divider" /><span className="auth-brand-caption">BILLING WORKSPACE</span></div>
        <div className="auth-story-copy"><span className="auth-eyebrow"><i /> SUBSCRIPTION OPERATIONS</span><h1>Every cycle,<br /><em>accounted for.</em></h1><p>Reliable recurring billing, explainable invoices, and a workspace that keeps every number connected to its source.</p>
          <div className="auth-feature-list"><div><span><Icon name="invoices" size={17} /></span><strong>Invoice Truth</strong><small>See how every total was calculated.</small></div><div><span><Icon name="reports" size={17} /></span><strong>Billing Control Tower</strong><small>Understand what needs attention.</small></div><div><span><Icon name="spark" size={17} /></span><strong>ORCA Copilot</strong><small>Move work forward with review built in.</small></div></div>
        </div>
      </div>
    </section>
    <section className="auth-main" aria-labelledby="auth-title">
      <div className="auth-card">
        <h2 id="auth-title">{title}</h2><p className="auth-description">{description}</p>
        {isLoading && <div className="auth-connection" role="status"><span className="auth-spinner" />Checking your secure workspace connection…</div>}
        {!isLoading && apiState === 'offline' && <div className="auth-connection auth-connection-error" role="alert"><Icon name="close" size={17} /><span>{connectionError || 'ORCA could not reach the API. Start the backend and database, then try again.'}</span><button type="button" onClick={onRetry}>Retry</button></div>}
        {!isLoading && apiState === 'online' && connectionError && <div className="auth-connection auth-connection-error" role="alert"><Icon name="close" size={17} /><span>{connectionError}</span><button type="button" onClick={onRetry}>Retry</button></div>}
        {!['forgot', 'reset'].includes(mode) && <div className="auth-mode-tabs" role="group" aria-label="Account type"><button type="button" aria-pressed={mode === 'login'} onClick={() => { setMode('login'); setError(''); setNotice(''); }}>Sign in</button><button type="button" aria-pressed={mode === 'signup'} onClick={() => { setMode('signup'); setError(''); setNotice(''); }}>Create workspace</button></div>}
        {mode === 'customer' && <div className="auth-context-note">Customer portal access is connected by your business and verified account email.</div>}
        <form className="auth-form" onSubmit={submit}>
          {mode === 'signup' && <label className="auth-field"><span>Business name</span><input name="businessName" autoComplete="organization" value={business} onChange={(event) => setBusiness(event.target.value)} placeholder="Your company" required disabled={inputDisabled} /></label>}
          {mode === 'customer' && <label className="auth-field"><span>Business ID</span><input name="businessId" value={business} onChange={(event) => setBusiness(event.target.value)} placeholder="From your invoice or invitation" required disabled={inputDisabled} /></label>}
          {mode !== 'reset' && <label className="auth-field"><span>Email address</span><input name="email" type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@company.com" required disabled={inputDisabled} /></label>}
          {mode !== 'forgot' && <label className="auth-field"><span>{mode === 'reset' ? 'New password' : 'Password'}{mode !== 'reset' && mode !== 'login' && <small>At least 9 characters</small>}</span><input name="password" type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} value={password} onChange={(event) => setPassword(event.target.value)} placeholder={mode === 'login' ? 'Enter your password' : 'Use uppercase, lowercase, and a special character'} minLength={mode === 'login' ? undefined : 9} required disabled={inputDisabled} /></label>}
          {mode !== 'login' && mode !== 'forgot' && mode !== 'reset' && <p className="auth-password-hint">Use at least 9 characters with uppercase, lowercase, and a special character.</p>}
          {error && <p className="auth-message auth-message-error" role="alert">{error}</p>}
          {notice && <p className="auth-message auth-message-success" role="status">{notice}</p>}
          <button className="auth-submit" type="submit" disabled={inputDisabled}>{busy ? <><span className="auth-spinner" />{mode === 'login' ? 'Signing in…' : 'Please wait…'}</> : mode === 'login' ? <>Sign in <Icon name="arrow" size={17} /></> : mode === 'signup' ? <>Create workspace <Icon name="arrow" size={17} /></> : mode === 'customer' ? <>Continue <Icon name="arrow" size={17} /></> : mode === 'forgot' ? <>Send reset link <Icon name="arrow" size={17} /></> : <>Update password <Icon name="arrow" size={17} /></>}</button>
        </form>
        <div className="auth-links">
          {mode === 'login' && <><button type="button" onClick={() => { setMode('forgot'); setError(''); setNotice(''); }}>Forgot password?</button><button type="button" onClick={() => { setMode('customer'); setBusiness(''); setError(''); setNotice(''); }}>Customer portal</button></>}
          {mode === 'customer' && <button type="button" onClick={() => { setMode('login'); setBusiness(''); setError(''); setNotice(''); }}>Back to sign in</button>}
          {(mode === 'forgot' || mode === 'reset') && <button type="button" onClick={() => { setMode('login'); setPassword(''); setError(''); setNotice(''); }}>Back to sign in</button>}
        </div>
      </div>
    </section>
  </main>;
}

function ReviewDialog({ value, onClose }: { value: { title: string; description: string; target: string; actionLabel: string; onApprove: () => Promise<boolean> }; onClose: () => void }) {
  const [busy, setBusy] = useState(false);
  return <Dialog title={value.title} description="Review the exact target and effect before this action is submitted." onClose={busy ? () => undefined : onClose}>
    <div className="review-summary"><span>Action</span><strong>{value.description}</strong><span>Target</span><strong>{value.target}</strong></div><div className="review-lock"><Icon name="shield" size={17} /><span>This review is recorded in ORCA. The server rechecks your access and the current record before applying it.</span></div><div className="form-actions"><Button variant="outline" onClick={onClose} disabled={busy}>Back</Button><Button variant="primary" icon="check" disabled={busy} onClick={async () => { setBusy(true); const done = await value.onApprove(); setBusy(false); if (done) onClose(); }}>{busy ? 'Submitting…' : value.actionLabel}</Button></div>
  </Dialog>;
}

type EditableVariant = { attributeName: string; value: string; adjustment: string };
type ProductEditValues = { name: string; type: string; price: string; cost: string; notes: string; taxRate: string; variants: EditableVariant[] };
type PlanEditValues = { name: string; price: string; cadence: string; minimumQuantity: string; availableFrom: string; availableUntil: string; taxRate: string; pauseAllowed: boolean; closeAllowed: boolean; autoClose: boolean; renewAllowed: boolean };

function ProductEditDialog({ product, onClose, onSave }: { product: Product; onClose: () => void; onSave: (values: ProductEditValues) => Promise<boolean> }) {
  const [name, setName] = useState(product.name);
  const [type, setType] = useState(product.type);
  const [price, setPrice] = useState(product.price);
  const [cost, setCost] = useState(product.cost);
  const [notes, setNotes] = useState(product.notes);
  const [taxRate, setTaxRate] = useState(percentageString(product.taxRateBps));
  const [variants, setVariants] = useState<EditableVariant[]>(() => product.variants.map((variant) => ({ attributeName: variant.attributeName ?? variant.name, value: variant.value, adjustment: variant.adjustment })));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const updateVariant = (index: number, key: keyof EditableVariant, value: string) => setVariants((items) => items.map((item, itemIndex) => itemIndex === index ? { ...item, [key]: value } : item));
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const kept = variants.filter((variant) => variant.attributeName.trim() || variant.value.trim());
    if (kept.some((variant) => !variant.attributeName.trim() || !variant.value.trim())) {
      setError('Each variant needs both an attribute and a value. Remove an unused row or complete it.');
      return;
    }
    try {
      kept.forEach((variant) => toPaise(variant.adjustment));
      setError(''); setBusy(true);
      const saved = await onSave({ name, type, price, cost, notes, taxRate, variants: kept.map((variant) => ({ ...variant, attributeName: variant.attributeName.trim(), value: variant.value.trim() })) });
      setBusy(false);
      if (saved) onClose();
    } catch (reason) {
      setBusy(false);
      setError(reason instanceof Error ? reason.message : 'Check the product values and try again.');
    }
  };
  return <Dialog title="Edit product" description={`Update ${product.name} and its future catalog pricing.`} onClose={busy ? () => undefined : onClose} wide>
    <Form onSubmit={(event) => void submit(event)}>
      <div className="form-grid">
        <TextField label="Product name" name="edit-product-name" value={name} onChange={setName} required />
        <SelectField label="Product type" name="edit-product-type" value={type} onChange={setType} options={[...new Set(['Software', 'Add-on', 'Service', 'Physical product', type])].map((value) => ({ label: value, value }))} required />
        <TextField label="Sales price · INR" name="edit-product-price" type="number" value={price} onChange={setPrice} required min="0" step="0.01" />
        <TextField label="Cost price · INR" name="edit-product-cost" type="number" value={cost} onChange={setCost} required min="0" step="0.01" />
        <TextField label="Tax rate (%)" name="edit-product-tax" type="number" value={taxRate} onChange={setTaxRate} required min="0" step="0.01" />
        <TextField label="Notes" name="edit-product-notes" value={notes} onChange={setNotes} />
      </div>
      <div className="variant-edit-heading"><div><strong>Variants</strong><span>Price adjustments are added to the product price.</span></div><Button variant="outline" size="sm" icon="plus" onClick={() => setVariants((items) => [...items, { attributeName: '', value: '', adjustment: '0.00' }])}>Add variant</Button></div>
      {variants.length > 0 && <div className="variant-edit-list">{variants.map((variant, index) => <div className="variant-edit-row" key={index}><TextField label={`Attribute ${index + 1}`} name={`edit-variant-attribute-${index}`} value={variant.attributeName} onChange={(value) => updateVariant(index, 'attributeName', value)} placeholder="e.g. Brand" /><TextField label={`Value ${index + 1}`} name={`edit-variant-value-${index}`} value={variant.value} onChange={(value) => updateVariant(index, 'value', value)} placeholder="e.g. Odoo" /><TextField label={`Adjustment · INR ${index + 1}`} name={`edit-variant-adjustment-${index}`} type="number" value={variant.adjustment} onChange={(value) => updateVariant(index, 'adjustment', value)} min="0" step="0.01" /><button type="button" className="small-action catalog-archive-action variant-remove-action" aria-label={`Remove variant ${variant.attributeName || index + 1}`} onClick={() => setVariants((items) => items.filter((_, itemIndex) => itemIndex !== index))}>Remove</button></div>)}</div>}
      {error && <p className="form-error" role="alert">{error}</p>}
      <FormActions onCancel={onClose} submit={busy ? 'Saving…' : 'Save product'} disabled={busy} />
    </Form>
  </Dialog>;
}

function PlanEditDialog({ plan, onClose, onSave }: { plan: Plan; onClose: () => void; onSave: (values: PlanEditValues) => Promise<boolean> }) {
  const [name, setName] = useState(plan.name);
  const [price, setPrice] = useState(plan.price);
  const [cadence, setCadence] = useState(plan.cadence);
  const [minimumQuantity, setMinimumQuantity] = useState(String(plan.minimumQuantity));
  const [availableFrom, setAvailableFrom] = useState(plan.availableFrom);
  const [availableUntil, setAvailableUntil] = useState(plan.availableUntil);
  const [taxRate, setTaxRate] = useState(percentageString(plan.taxRateBps));
  const [pauseAllowed, setPauseAllowed] = useState(plan.pauseAllowed);
  const [closeAllowed, setCloseAllowed] = useState(plan.closeAllowed);
  const [autoClose, setAutoClose] = useState(plan.autoClose ?? false);
  const [renewAllowed, setRenewAllowed] = useState(plan.renewAllowed);
  const [busy, setBusy] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true);
    const saved = await onSave({ name, price, cadence, minimumQuantity, availableFrom, availableUntil, taxRate, pauseAllowed, closeAllowed, autoClose, renewAllowed });
    setBusy(false); if (saved) onClose();
  };
  return <Dialog title="Edit plan" description={`Update ${plan.name} for future subscriptions and renewals.`} onClose={busy ? () => undefined : onClose}>
    <Form onSubmit={(event) => void submit(event)}>
      <div className="form-grid">
        <TextField label="Plan name" name="edit-plan-name" value={name} onChange={setName} required />
        <TextField label="Plan price · INR" name="edit-plan-price" type="number" value={price} onChange={setPrice} required min="0" step="0.01" />
        <SelectField label="Billing interval" name="edit-plan-interval" value={cadence} onChange={(value) => setCadence(value as Plan['cadence'])} options={['Daily', 'Weekly', 'Monthly', 'Yearly'].map((value) => ({ label: value, value }))} required />
        <TextField label="Minimum quantity" name="edit-plan-minimum" type="number" value={minimumQuantity} onChange={setMinimumQuantity} required min="1" step="1" />
        <TextField label="Tax rate (%)" name="edit-plan-tax" type="number" value={taxRate} onChange={setTaxRate} required min="0" step="0.01" />
        <TextField label="Available from" name="edit-plan-available-from" type="date" value={availableFrom} onChange={setAvailableFrom} />
        <TextField label="Available until" name="edit-plan-available-until" type="date" value={availableUntil} onChange={setAvailableUntil} />
      </div>
      <div className="form-toggle-stack">
        <ToggleField label="Allow pause" checked={pauseAllowed} onChange={setPauseAllowed} />
        <ToggleField label="Allow early close" checked={closeAllowed} onChange={setCloseAllowed} />
        <ToggleField label="Auto-close at term end" description="Automatically close the subscription when its term ends." checked={autoClose} onChange={setAutoClose} />
        <ToggleField label="Allow renewal" checked={renewAllowed} onChange={setRenewAllowed} />
      </div>
      <FormActions onCancel={onClose} submit={busy ? 'Saving…' : 'Save plan'} disabled={busy} />
    </Form>
  </Dialog>;
}

function CatalogArchiveDialog({ target, onClose, onArchive }: { target: CatalogArchiveTarget; onClose: () => void; onArchive: () => Promise<boolean> }) {
  const [busy, setBusy] = useState(false);
  const isProduct = target.kind === 'product';
  const name = target.record.name;
  return <Dialog title={`Archive ${isProduct ? 'product' : 'plan'}?`} description={`${name} will be removed from future catalog selection.`} onClose={busy ? () => undefined : onClose}>
    <div className="archive-explanation"><Icon name="shield" size={17} /><p>Existing subscriptions keep their stored terms. Archiving only stops this record from being selected for new subscriptions.</p></div>
    <div className="form-actions"><Button variant="outline" onClick={onClose} disabled={busy}>Cancel</Button><Button variant="danger" disabled={busy} onClick={async () => { setBusy(true); const archived = await onArchive(); setBusy(false); if (archived) onClose(); }}>{busy ? 'Archiving…' : `Archive ${isProduct ? 'product' : 'plan'}`}</Button></div>
  </Dialog>;
}

export default function App() {
  const { data, patch, apiState, dataMode, connectionError, toast, notify, commit, loadInvoice } = useWorkspace();
  const [route, setRoute] = useState<Route>(() => routeFromLocation());
  const [search, setSearch] = useState('');
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [isMobileViewport, setIsMobileViewport] = useState(() => window.matchMedia('(max-width: 700px)').matches);
  const mobileMenuButtonRef = useRef<HTMLButtonElement>(null);
  const sidebarRef = useRef<HTMLElement>(null);
  const wasMobileNavOpen = useRef(false);
  const [newMenuOpen, setNewMenuOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [recordDialog, setRecordDialog] = useState<CreateKind | null>(null);
  const [catalogEdit, setCatalogEdit] = useState<CatalogEditTarget | null>(null);
  const [catalogArchive, setCatalogArchive] = useState<CatalogArchiveTarget | null>(null);
  const [paymentInvoice, setPaymentInvoice] = useState<Invoice | null>(null);
  const [profile, setProfile] = useState<{ name: string; email: string; role: string; businessName: string }>({ name: '', email: '', role: 'unknown', businessName: '' });
  const [review, setReview] = useState<{ title: string; description: string; target: string; actionLabel: string; onApprove: () => Promise<boolean> } | null>(null);
  const [copilotBusy, setCopilotBusy] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const invoiceLoadAttempt = useRef('');
  useEffect(() => {
    try { localStorage.removeItem('orca-copilot-messages'); } catch { /* No local conversation is treated as a completed agent run. */ }
  }, []);
  useEffect(() => {
    const media = window.matchMedia('(max-width: 700px)');
    const updateViewport = () => {
      setIsMobileViewport(media.matches);
      if (!media.matches) setMobileNavOpen(false);
    };
    updateViewport();
    media.addEventListener('change', updateViewport);
    return () => media.removeEventListener('change', updateViewport);
  }, []);
  useLayoutEffect(() => {
    const sidebar = sidebarRef.current;
    if (!sidebar) return;
    if (isMobileViewport && !mobileNavOpen) sidebar.setAttribute('inert', '');
    else sidebar.removeAttribute('inert');
  }, [isMobileViewport, mobileNavOpen]);
  useEffect(() => {
    if (!isMobileViewport) {
      wasMobileNavOpen.current = false;
      return;
    }
    if (mobileNavOpen) {
      wasMobileNavOpen.current = true;
      sidebarRef.current?.querySelector<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled)')?.focus();
      const closeOnEscape = (event: KeyboardEvent) => {
        if (event.key === 'Escape') { event.preventDefault(); setMobileNavOpen(false); }
      };
      document.addEventListener('keydown', closeOnEscape);
      return () => document.removeEventListener('keydown', closeOnEscape);
    }
    if (wasMobileNavOpen.current) {
      wasMobileNavOpen.current = false;
      mobileMenuButtonRef.current?.focus();
    }
  }, [isMobileViewport, mobileNavOpen]);
  useEffect(() => {
    if (apiState !== 'online' || dataMode !== 'workspace') return;
    let active = true;
    void apiRequest<Record<string, unknown>>('/me').then((result) => {
      if (!active) return;
      const actor = result.actor && typeof result.actor === 'object' ? result.actor as Record<string, unknown> : {};
      const user = result.user && typeof result.user === 'object' ? result.user as Record<string, unknown> : {};
      const business = result.business && typeof result.business === 'object' ? result.business as Record<string, unknown> : {};
      const email = String(user.email ?? '');
      const name = String(user.name ?? email.split('@')[0] ?? 'Workspace member').replace(/[._-]+/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
      setProfile({ name, email, role: String(actor.role ?? 'staff'), businessName: String(business.name ?? 'Workspace') });
    }).catch(() => undefined);
    return () => { active = false; };
  }, [apiState, dataMode]);
  useEffect(() => {
    if (apiState !== 'online' || dataMode !== 'workspace') return;
    let active = true;
    void apiRequest<Record<string, unknown>>('/agent/runs').then((result) => {
      if (!active || !Array.isArray(result.items)) return;
      const restored: ChatMessage[] = [];
      const runs = [...result.items].filter((entry) => entry && typeof entry === 'object').map((entry) => entry as Record<string, unknown>).sort((left, right) => String(left.createdAt ?? '').localeCompare(String(right.createdAt ?? '')));
      for (const run of runs) {
        const stamp = run.createdAt ? new Date(String(run.createdAt)).toLocaleTimeString('en-IN', { hour: 'numeric', minute: '2-digit' }) : '';
        const transcript = Array.isArray(run.transcript) ? run.transcript : [];
        for (const entry of transcript) {
          if (!entry || typeof entry !== 'object') continue;
          const item = entry as Record<string, unknown>;
          const role = item.role === 'assistant' ? 'assistant' : item.role === 'user' ? 'user' : undefined;
          const content = typeof item.content === 'string' ? item.content : '';
          if (role && content) restored.push({ id: crypto.randomUUID(), role, text: content, time: stamp });
        }
        const response = String(run.message ?? '');
        if (response) restored.push({ id: crypto.randomUUID(), role: 'assistant', text: response, time: stamp });
      }
      setMessages(restored);
    }).catch(() => undefined);
    return () => { active = false; };
  }, [apiState, dataMode]);
  useEffect(() => {
    const pop = () => { setRoute(routeFromLocation()); setSearch(''); };
    window.addEventListener('popstate', pop); return () => window.removeEventListener('popstate', pop);
  }, []);
  useEffect(() => {
    if (route.page !== 'invoice' || !route.id) {
      invoiceLoadAttempt.current = '';
      return;
    }
    const invoice = data.invoices.find((item) => item.id === route.id);
    if (apiState === 'online' && dataMode === 'workspace' && (!invoice || !invoice.snapshot) && invoiceLoadAttempt.current !== route.id) {
      invoiceLoadAttempt.current = route.id;
      void loadInvoice(route.id);
    }
  }, [route.page, route.id, data.invoices, apiState, dataMode, loadInvoice]);

  const navigate = useCallback((page: PageKey, id?: string) => {
    const path = page === 'invoice' && id ? `${pagePaths.invoice}/${encodeURIComponent(id)}` : pagePaths[page];
    window.history.pushState({}, '', path); setRoute({ page, ...(id ? { id } : {}) }); setSearch(''); setMobileNavOpen(false); setProfileOpen(false);
  }, []);
  const signOut = async () => {
    try {
      await apiRequest('/auth/logout', { method: 'POST' });
      sessionStorage.removeItem('orca-csrf-token');
      window.location.reload();
    } catch (error) {
      notify(error instanceof Error ? `Could not sign out: ${error.message}` : 'Could not revoke this session.');
    }
  };
  const create = useCallback((kind: Parameters<PageActions['create']>[0]) => {
    if (kind === 'payment') {
      const invoice = data.invoices.find((item) => toPaise(outstanding(item)) > 0n && item.status === 'Confirmed');
      if (invoice) setPaymentInvoice(invoice); else notify('There are no open invoices to allocate a payment to.');
      return;
    }
    setRecordDialog(kind);
  }, [data.invoices, notify]);

  const openReview = (title: string, description: string, target: string, actionLabel: string, onApprove: () => Promise<boolean>) => setReview({ title, description, target, actionLabel, onApprove });

  const confirmInvoice = (invoice: Invoice) => openReview('Confirm invoice', `Finalize ${invoice.number} for ${money(invoice.total)}. This posts the invoice to the ledger.`, invoice.number, 'Confirm invoice', () => commit(`/invoices/${encodeURIComponent(invoice.id)}/confirm`, {}, () => patch('invoices', (items) => items.map((item) => item.id === invoice.id ? { ...item, status: 'Confirmed' } : item))));
  const cancelInvoice = (invoice: Invoice) => openReview('Cancel invoice', `Cancel ${invoice.number} and preserve its original calculation evidence.`, invoice.number, 'Cancel invoice', () => commit(`/invoices/${encodeURIComponent(invoice.id)}/cancel`, {}, () => patch('invoices', (items) => items.map((item) => item.id === invoice.id ? { ...item, status: 'Cancelled' } : item))));
  const sendInvoice = (invoice: Invoice) => {
    const customer = data.customers.find((item) => item.id === invoice.customerId);
    openReview('Queue invoice email', `Queue ${invoice.number} delivery to ${customer?.name ?? 'the customer'} at ${customer?.email ?? 'their account email'}. This records a delivery request; email delivery is not confirmed here.`, invoice.number, 'Queue email request', () => commit(`/invoices/${encodeURIComponent(invoice.id)}/send`, {}, () => undefined));
  };
  const openPayment = (invoice: Invoice) => setPaymentInvoice(invoice);
  const payOnline = async (invoice: Invoice) => {
    if (apiState !== 'online' || dataMode !== 'workspace' || profile.role !== 'customer') {
      throw new Error('Sign in to a customer account to pay an invoice online.');
    }
    if (invoice.status !== 'Confirmed' || toPaise(outstanding(invoice)) <= 0n) {
      throw new Error('This invoice has no confirmed outstanding balance to pay.');
    }
    try {
      const response = await apiRequest<unknown>(`/portal/invoices/${encodeURIComponent(invoice.id)}/checkout`, { method: 'POST' });
      window.location.assign(checkoutDestination(response));
    } catch (error) {
      throw new Error(onlinePaymentFailure(error));
    }
  };
  const transitionSubscription = (subscriptionId: string, action: string) => {
    const item = data.subscriptions.find((subscription) => subscription.id === subscriptionId);
    if (!item) return;
    const normalized = action.toLowerCase();
    const status = normalized.includes('quote') ? 'Quotation' : normalized === 'confirm' ? 'Confirmed' : normalized === 'activate' ? 'Active' : normalized.includes('pause') ? 'Paused' : normalized === 'resume' ? 'Active' : normalized === 'close' ? 'Closed' : normalized === 'renew' ? 'Draft' : item.status;
    if (status === item.status) return;
    const perform = () => commit(`/subscriptions/${encodeURIComponent(subscriptionId)}/transition`, { action: normalized.replaceAll(' ', '_') }, () => patch('subscriptions', (items) => items.map((subscription) => subscription.id === subscriptionId ? { ...subscription, status } : subscription)));
    if (status === 'Closed') openReview('Close subscription', `Close ${item.number} for ${money(item.amount)} recurring value.`, item.number, 'Approve & close', perform);
    else void perform();
  };
  const convertQuotation = (quotationId: string) => {
    const quote = data.quotations.find((item) => item.id === quotationId); if (!quote) return;
    const sub: WorkspaceData['subscriptions'][number] = { id: crypto.randomUUID(), number: `SUB-${1049 + data.subscriptions.length}`, customerId: quote.customerId, planId: data.plans[0]?.id ?? '', status: 'Draft', startedAt: new Date().toISOString().slice(0, 10), renewsAt: '', amount: quote.amount, seats: 1, paymentTerms: 'Net 30' };
    void commit('/subscriptions', { quotationId, customerId: quote.customerId }, (response) => {
      const result = response && typeof response === 'object' ? response as Record<string, unknown> : {};
      const lines = Array.isArray(result.lines) ? result.lines as Record<string, unknown>[] : [];
      const terms = Number(result.paymentTermsDays ?? 30);
      const created = {
        ...sub,
        id: String(result.id ?? sub.id),
        number: String(result.number ?? sub.number),
        planId: String(result.planId ?? sub.planId),
        status: String(result.status ?? sub.status) as WorkspaceData['subscriptions'][number]['status'],
        startedAt: String(result.startDate ?? sub.startedAt),
        renewsAt: String(result.expiryDate ?? ''),
        amount: typeof result.amount === 'string' ? result.amount : quote.amount,
        seats: Math.max(1, Number(lines[0]?.quantity ?? 1)),
        paymentTerms: terms === 0 ? 'Due on receipt' : `Net ${terms}`,
      };
      patch('subscriptions', (items) => [...items, created]);
      patch('quotations', (items) => items.map((item) => item.id === quote.id ? { ...item, status: 'Converted' } : item));
      navigate('subscriptions');
    });
  };
  const acceptQuotation = (quotationId: string) => {
    const quote = data.quotations.find((item) => item.id === quotationId);
    if (!quote || quote.status !== 'Sent' || quote.validUntil < new Date().toISOString().slice(0, 10)) return;
    void commit(`/quotations/${encodeURIComponent(quotationId)}/accept`, {}, () => {
      patch('quotations', (items) => items.map((item) => item.id === quote.id ? { ...item, status: 'Accepted' } : item));
    });
  };
  const exportData = async () => {
    if (apiState !== 'online' || dataMode !== 'workspace') {
      notify('Connect to the API and sign in before requesting an export.');
      return;
    }
    const fallbackId = crypto.randomUUID();
    await commit('/exports', { kind: 'full', format: 'zip' }, (response) => {
      const id = responseId(response, fallbackId);
      patch('exports', (items) => [{ id, name: 'Full business archive', createdAt: new Date().toISOString(), status: 'Queued' }, ...items]);
    });
  };
  const downloadExport = async (jobId: string) => {
    if (apiState !== 'online' || dataMode !== 'workspace') {
      notify('Connect to the API and sign in before downloading an export.'); return;
    }
    try {
      const response = await fetch(`/api/v1/exports/${encodeURIComponent(jobId)}/download`, { credentials: 'include', cache: 'no-store' });
      if (!response.ok) throw new Error(response.status === 409 ? 'This export is still preparing.' : 'The export could not be downloaded.');
      const blob = await response.blob(); const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = `orca-export-${jobId}.zip`; link.click(); URL.revokeObjectURL(url);
    } catch (error) { notify(error instanceof Error ? error.message : 'The export could not be downloaded.'); }
  };
  const runBilling = async (): Promise<number | null> => {
    if (apiState !== 'online' || dataMode !== 'workspace' || !['admin', 'staff'].includes(profile.role)) {
      notify('Sign in with workspace staff access before running billing.');
      return null;
    }
    try {
      const result = await apiRequest<Record<string, unknown>>('/billing/run', { method: 'POST', body: '{}' });
      const created = Number(result.created);
      if (!Number.isSafeInteger(created) || created < 0) throw new Error('The API returned an invalid billing result.');
      return created;
    } catch (error) {
      notify(error instanceof Error ? `Billing run failed: ${error.message}` : 'Billing could not be run. Try again.');
      return null;
    }
  };
  const decideApproval = (approval: Approval, decision: 'approve' | 'reject') => {
    openReview(decision === 'approve' ? 'Approve and execute proposal' : 'Decline proposed action', `${decision === 'approve' ? 'Execute' : 'Decline'} “${approval.action}” with the exact inputs below. The server will recheck permissions and record versions before it runs. ${approval.detail}`, approval.target, decision === 'approve' ? 'Approve and execute' : 'Decline proposal', () => commit(`/agent/approvals/${encodeURIComponent(approval.id)}/decision`, { decision }, (response) => {
      const result = response && typeof response === 'object' && 'executionResult' in response && (response as { executionResult?: unknown }).executionResult && typeof (response as { executionResult?: unknown }).executionResult === 'object'
        ? (response as { executionResult: Record<string, unknown> }).executionResult
        : undefined;
      patch('approvals', (items) => items.map((item) => item.id === approval.id ? { ...item, status: decision === 'approve' ? 'Approved' : 'Declined', ...(result ? { executionResult: result } : {}) } : item));
      if (decision !== 'approve' || !result) return;
      const resourceId = String(result.resourceId ?? approval.target);
      if (result.resourceType === 'invoice') {
        const invoice = data.invoices.find((item) => item.id === resourceId || item.number === resourceId);
        if (!invoice) return;
        const status = result.status;
        const remaining = typeof result.outstanding === 'string' ? result.outstanding : status === 'Paid' ? '0.00' : undefined;
        const paid = remaining ? fromPaise(toPaise(invoice.total) - toPaise(remaining)) : invoice.paid;
        patch('invoices', (items) => items.map((item) => item.id === invoice.id ? { ...item, ...(typeof status === 'string' ? { status: status as Invoice['status'] } : {}), ...(remaining ? { paid } : {}) } : item));
        if (typeof result.paymentId === 'string' && typeof result.amount === 'string') {
          patch('payments', (items) => [{ id: result.paymentId as string, invoiceId: invoice.id, customerId: invoice.customerId, amount: result.amount as string, date: typeof result.receivedAt === 'string' ? result.receivedAt.slice(0, 10) : new Date().toISOString().slice(0, 10), method: typeof result.method === 'string' ? result.method : '', note: typeof result.note === 'string' ? result.note : '' }, ...items]);
        }
      } else if (result.resourceType === 'subscription' && typeof result.status === 'string') {
        patch('subscriptions', (items) => items.map((item) => item.id === resourceId ? { ...item, status: result.status as WorkspaceData['subscriptions'][number]['status'] } : item));
      }
    }));
  };
  const sendCopilot = async (prompt: string) => {
    const stamp = new Date().toLocaleTimeString('en-IN', { hour: 'numeric', minute: '2-digit' });
    setMessages((items) => [...items, { id: crypto.randomUUID(), role: 'user', text: prompt, time: stamp }]);
    if (apiState !== 'online' || dataMode !== 'workspace') {
      notify(apiState === 'offline' ? 'Copilot is unavailable offline. No request was sent.' : 'Sign in to a workspace to run Copilot. No request was sent.');
      return;
    }
    setCopilotBusy(true);
    try {
      const result = await apiRequest<Record<string, unknown>>('/agent/runs', { method: 'POST', body: JSON.stringify({ message: prompt }) });
      const runId = String(result.id ?? (result.run as Record<string, unknown> | undefined)?.id ?? '');
      let outcome = result;
      if (runId) {
        for (let attempt = 0; attempt < 10; attempt++) {
          await new Promise((resolve) => window.setTimeout(resolve, 900));
          outcome = await apiRequest<Record<string, unknown>>(`/agent/runs/${encodeURIComponent(runId)}`);
          if (['completed', 'failed', 'cancelled', 'needs_review'].includes(String(outcome.status).toLowerCase())) break;
        }
      }
      const responseText = String(outcome.message ?? outcome.summary ?? outcome.response ?? (runId ? `Run ${runId} is ${String(outcome.status ?? 'in progress')}.` : 'The workspace accepted this Copilot run.'));
      setMessages((items) => [...items, { id: crypto.randomUUID(), role: 'assistant', text: responseText, time: new Date().toLocaleTimeString('en-IN', { hour: 'numeric', minute: '2-digit' }) }]);
    } catch (error) {
      notify(error instanceof Error ? `Copilot run failed: ${error.message}` : 'Copilot run failed. No action was completed.');
    } finally { setCopilotBusy(false); }
  };

  const saveRecord = async (kind: NonNullable<typeof recordDialog>, values: Record<string, string | boolean>): Promise<boolean> => {
    const value = (key: string) => String(values[key] ?? '');
    try {
      const id = crypto.randomUUID();
      if (kind === 'customer') {
        const name = value('name').trim(); const company = value('company').trim(); const email = value('email').trim();
        if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) throw new Error('Enter a valid email address.');
        const customer = { id, name, email, company, phone: value('phone'), status: 'Active' as const, since: new Date().toISOString().slice(0, 10), initials: initials(name) };
        return await commit('/customers', { name, company, email, phone: customer.phone }, (response) => patch('customers', (items) => [...items, { ...customer, id: responseId(response, id) }]));
      }
      if (kind === 'product') {
        const price = fromPaise(toPaise(value('price'))); const cost = fromPaise(toPaise(value('cost'))); const adjustment = fromPaise(toPaise(value('variantAdjustment') || '0.00'));
        const variantAttribute = value('variantAttribute').trim(); const variantValue = value('variantValue').trim();
        if (Boolean(variantAttribute) !== Boolean(variantValue)) throw new Error('Enter both the variant attribute and value, or leave both blank.');
        const variants = variantAttribute ? [{ name: variantAttribute === variantValue ? variantAttribute : `${variantAttribute} · ${variantValue}`, attributeName: variantAttribute, value: variantValue, adjustment }] : [];
        const taxRateBps = percentageBps(value('taxRate'));
        const product = { id, name: value('name').trim(), type: value('type'), price, cost, notes: value('notes'), variants, taxRateBps, active: true, version: 1 };
        const body = { name: product.name, type: product.type, salesPrice: price, costPrice: cost, currency: 'INR', notes: product.notes, taxRate: percentageString(taxRateBps), variants: variants.map((variant) => ({ name: variant.attributeName, value: variant.value, adjustment: variant.adjustment })) };
        return await commit('/products', body, (response) => patch('products', (items) => [...items, { ...product, id: responseId(response, id) }]));
      }
      if (kind === 'plan') {
        const taxRateBps = percentageBps(value('taxRate'));
        const plan = { id, name: value('name').trim(), price: fromPaise(toPaise(value('price'))), cadence: value('cadence') as WorkspaceData['plans'][number]['cadence'], minimumQuantity: Math.max(1, Number(value('minimumQuantity'))), availableFrom: value('availableFrom'), availableUntil: value('availableUntil'), pauseAllowed: Boolean(values.pauseAllowed), closeAllowed: Boolean(values.closeAllowed), autoClose: Boolean(values.autoClose), renewAllowed: Boolean(values.renewAllowed), taxRateBps, active: true, version: 1 };
        const body = { name: plan.name, salesPrice: plan.price, interval: plan.cadence.toLowerCase(), minimumQuantity: plan.minimumQuantity, availableFrom: plan.availableFrom, availableUntil: plan.availableUntil, taxRate: percentageString(taxRateBps), autoClose: plan.autoClose, allowPause: plan.pauseAllowed, allowEarlyClose: plan.closeAllowed, allowRenew: plan.renewAllowed };
        return await commit('/plans', body, (response) => patch('plans', (items) => [...items, { ...plan, id: responseId(response, id) }]));
      }
      if (kind === 'quoteTemplate') {
        const plan = data.plans.find((item) => item.id === value('templatePlanId'));
        if (!plan) throw new Error('Choose a billing plan for this template.');
        const quantity = Number(value('templateQuantity'));
        if (!Number.isSafeInteger(quantity) || quantity < plan.minimumQuantity) throw new Error(`${plan.name} requires a minimum quantity of ${plan.minimumQuantity}.`);
        const validDays = Number(value('validDays'));
        if (!Number.isSafeInteger(validDays) || validDays < 1 || validDays > 365) throw new Error('Template validity must be between 1 and 365 days.');
        const product = data.products.find((item) => item.id === value('templateProductId'));
        const productVariant = product?.variants.find((variant) => variant.value === value('templateProductVariant'));
        const productUnitPrice = product ? addMoney(product.price, productVariant?.adjustment ?? '0.00') : '0.00';
        const template: WorkspaceData['quoteTemplates'][number] = {
          id, name: value('name').trim(), validDays, planId: plan.id,
          lines: [
            { productId: '', name: plan.name, quantity, unitPrice: plan.price, taxRate: percentageString(plan.taxRateBps) },
            ...(product ? [{ productId: product.id, name: product.name, quantity: 1, unitPrice: productUnitPrice, taxRate: percentageString(product.taxRateBps), ...(productVariant ? { variant: productVariant.value } : {}) }] : []),
          ],
        };
        return await commit('/quotation-templates', { name: template.name, validDays, planId: plan.id, productId: product?.id, productVariant: productVariant?.value, quantity }, (response) => patch('quoteTemplates', (items) => [...items, { ...template, id: responseId(response, id) }]));
      }
      if (kind === 'quotation' || kind === 'subscription') {
        const customerId = value('customerId');
        const template = kind === 'quotation' ? data.quoteTemplates.find((item) => item.id === value('templateId')) : undefined;
        const plan = data.plans.find((item) => item.id === (template?.planId || value('planId'))) ?? data.plans[0];
        if (!plan || !customerId) throw new Error('Choose a customer and plan.');
        const quantity = Number(template?.lines.find((line) => !line.productId)?.quantity ?? value('quantity'));
        if (!Number.isSafeInteger(quantity) || quantity < plan.minimumQuantity) throw new Error(`${plan.name} requires a minimum quantity of ${plan.minimumQuantity}.`);
        const product = kind === 'quotation' && template ? undefined : data.products.find((item) => item.id === value('productId'));
        const productVariant = product?.variants.find((variant) => variant.value === value('productVariant'));
        const productQuantity = Number(value('productQuantity') || '1');
        if (product && (!Number.isSafeInteger(productQuantity) || productQuantity < 1)) throw new Error('Add-on quantity must be a positive whole number.');
        const productUnitPrice = product ? addMoney(product.price, productVariant?.adjustment ?? '0.00') : '0.00';
        const productLine = product ? { productId: product.id, name: product.name, quantity: productQuantity, unitPrice: productUnitPrice, taxRate: percentageString(product.taxRateBps), ...(productVariant ? { variant: productVariant.value } : {}) } : undefined;
        const planLine = { productId: '', name: plan.name, quantity, unitPrice: plan.price, taxRate: percentageString(plan.taxRateBps) };
        const directLines = [planLine, ...(productLine ? [productLine] : [])];
        if (kind === 'quotation') {
          const validDays = template?.validDays ?? Number(value('validDays'));
          const templateAmount = template?.lines.reduce((sum, line) => addMoney(sum, multiplyMoney(line.unitPrice, line.quantity)), '0.00');
          const amount = templateAmount ?? directLines.reduce((sum, line) => addMoney(sum, multiplyMoney(line.unitPrice, line.quantity)), '0.00');
          const quote = { id, number: `QUO-2026-${String(data.quotations.length + 15).padStart(3, '0')}`, customerId, title: value('title').trim() || `${template?.name ?? plan.name} · ${template ? 'offer' : `${quantity} ${quantity === 1 ? 'seat' : 'seats'}`}`, date: new Date().toISOString().slice(0, 10), validUntil: new Date(Date.now() + validDays * 86400000).toISOString().slice(0, 10), amount, status: 'Draft' };
          const quoteLines = template ? undefined : directLines;
          const body = { customerId, planId: plan.id, title: quote.title, validDays, quantity, templateId: template?.id, lines: quoteLines };
          return await commit('/quotations', body, (response) => {
            const saved = response && typeof response === 'object' ? response as Record<string, unknown> : {};
            patch('quotations', (items) => [...items, {
              ...quote, id: responseId(response, id),
              number: String(saved.number ?? quote.number), title: String(saved.title ?? quote.title),
              date: String(saved.date ?? quote.date), validUntil: String(saved.validUntil ?? quote.validUntil),
              amount: String(saved.amount ?? quote.amount), status: String(saved.status ?? quote.status),
            }]);
          });
        }
        const amount = directLines.reduce((sum, line) => addMoney(sum, multiplyMoney(line.unitPrice, line.quantity)), '0.00');
        const subscription = { id, number: `SUB-${1049 + data.subscriptions.length}`, customerId, planId: plan.id, status: 'Draft' as const, startedAt: value('availableFrom'), renewsAt: value('availableUntil'), amount, seats: quantity, paymentTerms: value('paymentTerms') === '0' ? 'Due on receipt' : `Net ${value('paymentTerms')}` };
        const body = { customerId, planId: plan.id, startDate: subscription.startedAt, expiryDate: subscription.renewsAt || undefined, paymentTermsDays: Number(value('paymentTerms')), lines: directLines };
        return await commit('/subscriptions', body, (response) => patch('subscriptions', (items) => [...items, { ...subscription, id: responseId(response, id) }]));
      }
      if (kind === 'discount') {
        const discountKind = value('discountKind') as 'Fixed' | 'Percentage';
        const dateBoundary = (date: string, endOfDay = false) => date ? `${date}T${endOfDay ? '23:59:59.999' : '00:00:00.000'}Z` : '';
        const startsAt = value('startsAt'); const endsAt = value('endsAt');
        if (startsAt && endsAt && startsAt > endsAt) throw new Error('The end date must be on or after the start date.');
        const discount = { id, name: value('name').trim(), kind: discountKind, value: discountKind === 'Fixed' ? fromPaise(toPaise(value('value'))) : value('value'), scope: value('scope'), minimum: fromPaise(toPaise(value('minimum'))), minimumQuantity: Math.max(1, Number(value('minimumQuantity'))), usage: 0, limit: Number(value('usageLimit')), startsAt, endsAt, active: true };
        const scopedProductId = discount.scope === 'Subscriptions' ? '' : discount.scope;
        const body = { name: discount.name, kind: discountKind.toLowerCase(), value: discount.value, minimumPurchase: discount.minimum, minimumQuantity: discount.minimumQuantity, scope: scopedProductId ? 'product' : 'subscription', productIds: scopedProductId ? [scopedProductId] : [], maxUses: discount.limit, startsAt: dateBoundary(startsAt), endsAt: dateBoundary(endsAt, true) };
        return await commit('/discounts', body, (response) => patch('discounts', (items) => [...items, { ...discount, id: responseId(response, id) }]));
      }
      if (kind === 'staff') {
        const email = value('email').trim();
        const name = value('name').trim() || email;
        const staff = { id, name, email, role: 'Staff' as const, status: 'Active', initials: initials(name) };
        return await commit('/staff', { name, email: staff.email, role: 'staff' }, (response) => patch('staff', (items) => [...items, { ...staff, id: responseId(response, id) }]));
      }
      const rate = value('rate');
      const tax: TaxRule = { id, name: value('name').trim(), region: value('region').trim(), rateBps: percentageBps(rate), active: true };
      return await commit('/taxes', { name: tax.name, region: tax.region, rate: `${Math.floor(tax.rateBps / 100)}.${String(tax.rateBps % 100).padStart(2, '0')}` }, (response) => patch('taxes', (items) => [...items, { ...tax, id: responseId(response, id) }]));
    } catch (error) { notify(error instanceof Error ? error.message : 'Check the form fields and try again.'); return false; }
  };

  const savePayment = (invoice: Invoice, values: { amount: string; method: string; date: string; note: string }) => new Promise<boolean>((resolve) => {
    const operationKey = crypto.randomUUID();
    openReview('Review payment allocation', `Record ${money(values.amount)} by ${values.method} on ${formatDate(new Date(`${values.date}T12:00:00`))}.`, invoice.number, 'Approve & record payment', async () => {
      const done = await commit(`/invoices/${encodeURIComponent(invoice.id)}/payments`, { ...values, operationKey }, () => {
        const updated = data.invoices.find((item) => item.id === invoice.id);
        if (!updated) return;
        const paid = addMoney(updated.paid, values.amount);
        const status = toPaise(paid) >= toPaise(updated.total) ? 'Paid' : updated.status;
        patch('invoices', (items) => items.map((item) => item.id === invoice.id ? { ...item, paid, status } : item));
        patch('payments', (items) => [...items, { id: crypto.randomUUID(), invoiceId: invoice.id, customerId: invoice.customerId, amount: values.amount, date: values.date, method: values.method, note: values.note }]);
      });
      resolve(done);
      return done;
    });
  });

  const saveProduct = async (product: Product, values: ProductEditValues): Promise<boolean> => {
    try {
      const salesPrice = fromPaise(toPaise(values.price));
      const costPrice = fromPaise(toPaise(values.cost));
      const taxRateBps = percentageBps(values.taxRate);
      const variants = values.variants.map((variant) => ({
        name: variant.attributeName,
        value: variant.value,
        adjustment: fromPaise(toPaise(variant.adjustment)),
      }));
      const expectedVersion = product.version ?? 1;
      const response = await apiRequest<Record<string, unknown>>(`/products/${encodeURIComponent(product.id)}`, {
        method: 'PUT',
        body: JSON.stringify({ expectedVersion, name: values.name.trim(), type: values.type, salesPrice, costPrice, notes: values.notes.trim(), taxRate: percentageString(taxRateBps), variants }),
      });
      const updated: Product = {
        ...product, name: values.name.trim(), type: values.type, price: salesPrice, cost: costPrice,
        notes: values.notes.trim(), taxRateBps,
        variants: variants.map((variant) => ({ ...variant, attributeName: variant.name, name: variant.name === variant.value ? variant.name : `${variant.name} · ${variant.value}` })),
        version: Math.max(expectedVersion + 1, Number(response.version ?? expectedVersion + 1)), active: true,
      };
      patch('products', (items) => items.map((item) => item.id === product.id ? updated : item));
      notify(`Product “${updated.name}” saved.`);
      return true;
    } catch (error) {
      const message = error instanceof ApiError && error.status === 409
        ? 'Not saved: this product changed elsewhere. Refresh the workspace and review the latest version.'
        : error instanceof Error ? `Product not saved: ${error.message}` : 'Product not saved. Check the connection and try again.';
      notify(message);
      return false;
    }
  };

  const savePlan = async (plan: Plan, values: PlanEditValues): Promise<boolean> => {
    try {
      const salesPrice = fromPaise(toPaise(values.price));
      const taxRateBps = percentageBps(values.taxRate);
      const minimumQuantity = Number(values.minimumQuantity);
      if (!Number.isSafeInteger(minimumQuantity) || minimumQuantity < 1) throw new Error('Minimum quantity must be a positive whole number.');
      const expectedVersion = plan.version ?? 1;
      const response = await apiRequest<Record<string, unknown>>(`/plans/${encodeURIComponent(plan.id)}`, {
        method: 'PUT',
        body: JSON.stringify({
          expectedVersion, name: values.name.trim(), interval: values.cadence.toLowerCase(), salesPrice, minimumQuantity,
          availableFrom: values.availableFrom, availableUntil: values.availableUntil, taxRate: percentageString(taxRateBps),
          autoClose: values.autoClose, allowEarlyClose: values.closeAllowed, allowPause: values.pauseAllowed, allowRenew: values.renewAllowed,
        }),
      });
      const updated: Plan = {
        ...plan, name: values.name.trim(), price: salesPrice, cadence: values.cadence as Plan['cadence'], minimumQuantity,
        availableFrom: values.availableFrom, availableUntil: values.availableUntil, taxRateBps,
        pauseAllowed: values.pauseAllowed, closeAllowed: values.closeAllowed, autoClose: values.autoClose,
        renewAllowed: values.renewAllowed, version: Math.max(expectedVersion + 1, Number(response.version ?? expectedVersion + 1)), active: true,
      };
      patch('plans', (items) => items.map((item) => item.id === plan.id ? updated : item));
      notify(`Plan “${updated.name}” saved.`);
      return true;
    } catch (error) {
      const message = error instanceof ApiError && error.status === 409
        ? 'Not saved: this plan changed elsewhere. Refresh the workspace and review the latest version.'
        : error instanceof Error ? `Plan not saved: ${error.message}` : 'Plan not saved. Check the connection and try again.';
      notify(message);
      return false;
    }
  };

  const archiveCatalogRecord = async (target: CatalogArchiveTarget): Promise<boolean> => {
    const kindName = target.kind === 'product' ? 'Product' : 'Plan';
    const path = target.kind === 'product' ? 'products' : 'plans';
    try {
      await apiRequest(`/${path}/${encodeURIComponent(target.record.id)}/archive`, {
        method: 'POST', body: JSON.stringify({ expectedVersion: target.record.version ?? 1 }),
      });
      if (target.kind === 'product') patch('products', (items) => items.filter((item) => item.id !== target.record.id));
      else patch('plans', (items) => items.filter((item) => item.id !== target.record.id));
      notify(`${kindName} archived. Existing subscriptions keep their stored terms.`);
      return true;
    } catch (error) {
      const message = error instanceof ApiError && error.status === 409
        ? `Not archived: this ${target.kind} changed elsewhere. Refresh the workspace and review the latest version.`
        : error instanceof Error ? `${kindName} not archived: ${error.message}` : `${kindName} not archived. Check the connection and try again.`;
      notify(message);
      return false;
    }
  };

  const actions: PageActions = {
    navigate, create, confirmInvoice, cancelInvoice, sendInvoice, openPayment, payOnline, transitionSubscription, convertQuotation, acceptQuotation, exportData, downloadExport, runBilling, decideApproval, search, setSearch,
    canManageCatalog: profile.role === 'admin',
    editProduct: (product) => setCatalogEdit({ kind: 'product', record: product }),
    archiveProduct: (product) => setCatalogArchive({ kind: 'product', record: product }),
    editPlan: (plan) => setCatalogEdit({ kind: 'plan', record: plan }),
    archivePlan: (plan) => setCatalogArchive({ kind: 'plan', record: plan }),
    sendQuotation: (id: string) => {
      const quote = data.quotations.find((item) => item.id === id); if (!quote) return;
      const customer = data.customers.find((item) => item.id === quote.customerId);
      openReview('Queue quotation email', `Queue ${quote.number} delivery to ${customer?.email ?? 'the customer email on file'}. This records a delivery request; email delivery is not confirmed here.`, quote.number, 'Queue email request', () => commit(`/quotations/${encodeURIComponent(id)}/send`, {}, () => patch('quotations', (items) => items.map((item) => item.id === id ? { ...item, status: 'Sent' } : item))));
    },
  };

  const isCustomer = dataMode === 'workspace' && profile.role === 'customer';
  const canManageInvoices = dataMode === 'workspace' && ['admin', 'staff'].includes(profile.role);
  const canPayOnline = isCustomer && apiState === 'online';
  const page = (() => {
    switch (route.page) {
      case 'overview': return <OverviewPage data={data} actions={actions} canRunBilling={['admin', 'staff'].includes(profile.role)} />;
      case 'customers': return <CustomersPage data={data} actions={actions} />;
      case 'catalog': return <CatalogPage data={data} actions={actions} />;
      case 'plans': return <PlansPage data={data} actions={actions} />;
      case 'quotations': return <QuotationsPage data={data} actions={actions} />;
      case 'subscriptions': return <SubscriptionsPage data={data} actions={actions} />;
      case 'invoices': return <InvoicesPage data={data} actions={actions} />;
      case 'invoice': return <InvoiceTruthPage data={data} invoiceId={route.id ?? ''} businessName={profile.businessName} actions={actions} isCustomer={isCustomer} canManageInvoices={canManageInvoices} canPayOnline={canPayOnline} />;
      case 'payments': return <PaymentsPage data={data} actions={actions} />;
      case 'reports': return <ReportsPage data={data} />;
      case 'discounts': return <DiscountsPage data={data} actions={actions} />;
      case 'taxes': return <TaxesPage data={data} actions={actions} />;
      case 'staff': return <StaffPage data={data} actions={actions} />;
      case 'exports': return <ExportsPage data={data} actions={actions} />;
      case 'copilot': return <CopilotPage data={data} messages={messages} onSend={sendCopilot} actions={actions} busy={copilotBusy} canRun={apiState === 'online' && dataMode === 'workspace'} canReview={apiState === 'online' && dataMode === 'workspace' && ['admin', 'staff'].includes(profile.role)} />;
      case 'portal': return <PortalPage data={data} actions={actions} isCustomer={isCustomer} canPayOnline={canPayOnline} />;
      default: return <SettingsPage actions={actions} />;
    }
  })();

  const apiLabel = 'Workspace connected';
  const apiTone = 'green';
  const workspaceTitle = profile.businessName;
  const userName = profile.name || profile.email;
  const userRole = profile.role === 'admin' ? 'Workspace admin' : profile.role === 'staff' ? 'Staff member' : 'Customer';
  const userInitials = initials(profile.name || profile.email || 'Workspace');
  const newRecordKinds: CreateKind[] = profile.role === 'admin' ? ['customer', 'product', 'plan', 'quotation', 'subscription'] : ['customer', 'quotation', 'subscription'];
  const toastIsError = Boolean(toast && /not saved|not archived|failed|could not|cannot|unavailable|offline|expired|no changes were saved/i.test(toast));
  if (apiState !== 'online' || dataMode !== 'workspace') {
    return <AuthScreen apiState={apiState} connectionError={connectionError} onRetry={() => window.location.reload()} onAuthenticated={() => window.location.reload()} />;
  }
  return <div className={`app-shell ${mobileNavOpen ? 'nav-open' : ''}`}>
    <aside id="orca-sidebar" className="sidebar" ref={sidebarRef} aria-label="Workspace sidebar" aria-hidden={isMobileViewport && !mobileNavOpen}>
      <button type="button" className="brand-lockup" onClick={() => navigate('overview')} aria-label="ORCA home"><span className="brand-mark"><Icon name="overview" size={19} /></span><span><strong>orca</strong><small>Billing workspace</small></span></button>
      <div className="workspace-switch"><span className="workspace-avatar">{workspaceTitle.slice(0, 1).toUpperCase()}</span><span><strong>{workspaceTitle}</strong><small>Business workspace</small></span><Icon name="chevron" size={15} /></div>
      <div className="side-section-label">WORKSPACE</div>
      <nav className="primary-nav" aria-label="Main navigation">{primaryNav.map((item) => <button key={item.page} type="button" className={`nav-item ${route.page === item.page || (route.page === 'invoice' && item.page === 'invoices') ? 'active-nav' : ''}`} onClick={() => navigate(item.page)}><Icon name={item.icon} size={18} /><span>{pageTitles[item.page]}</span>{item.page === 'invoices' && <span className="nav-count">{data.invoices.filter((invoice) => toPaise(outstanding(invoice)) > 0n).length}</span>}</button>)}</nav>
      <div className="side-divider" /><div className="side-section-label">WORKSPACE TOOLS</div>
      <button type="button" className={`nav-item ${route.page === 'copilot' ? 'active-nav copilot-nav' : 'copilot-nav'}`} onClick={() => navigate('copilot')}><span className="side-spark"><Icon name="spark" size={17} /></span><span>ORCA Copilot</span><span className="nav-ping" /></button>
      <button type="button" className={`nav-item ${['settings', 'discounts', 'taxes', 'staff', 'exports'].includes(route.page) ? 'active-nav' : ''}`} onClick={() => navigate('settings')}><Icon name="settings" size={18} /><span>Settings</span></button>
      <div className="sidebar-spacer" />
      <div className="sidebar-help"><span className="help-orb"><Icon name="shield" size={17} /></span><strong>Billing, in balance.</strong><small>Your records stay in sync across every billing cycle.</small><button type="button" className="help-link" onClick={() => navigate('reports')}>View billing health <Icon name="arrow" size={13} /></button></div>
      <button type="button" className="portal-shortcut" onClick={() => navigate('portal')}><Icon name="globe" size={16} />Customer portal<Icon name="arrow" size={13} /></button>
      <button type="button" className="sidebar-profile" onClick={() => setProfileOpen(!profileOpen)}><Avatar initials={userInitials} size="sm" /><span><strong>{userName}</strong><small>{userRole}</small></span><Icon name="more" size={16} /></button>
      {profileOpen && <div className="profile-popover"><button type="button" onClick={() => navigate('portal')}>Customer portal</button><button type="button" onClick={() => navigate('settings')}>Workspace settings</button><button type="button" onClick={() => void signOut()}>Sign out</button></div>}
    </aside>
    {mobileNavOpen && <button type="button" className="nav-scrim" aria-label="Close navigation" onClick={() => setMobileNavOpen(false)} />}
    <div className="main-shell">
      <header className="topbar"><div className="topbar-left"><button ref={mobileMenuButtonRef} type="button" className="icon-button mobile-menu-button" aria-label="Open navigation" aria-expanded={mobileNavOpen} aria-controls="orca-sidebar" onClick={() => setMobileNavOpen(true)}><Icon name="menu" size={18} /></button><div className="breadcrumb"><span>{profile.businessName}</span><Icon name="chevron" size={14} /><strong>{pageTitles[route.page]}</strong></div></div><div className="topbar-actions"><label className="global-search"><Icon name="search" size={16} /><span className="sr-only">Search the current page</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search anything…" /><kbd>⌘ K</kbd></label><Badge tone={apiTone}>{apiLabel}</Badge><div className="topbar-divider" /><IconButton icon="bell" label="Open Copilot approvals" onClick={() => navigate('copilot')} /><div className="top-avatar"><Avatar initials={userInitials} size="sm" /></div><div className="new-menu-wrap"><Button variant="primary" size="sm" icon="plus" onClick={() => setNewMenuOpen(!newMenuOpen)}>New</Button>{newMenuOpen && <div className="new-menu">{newRecordKinds.map((kind) => <button type="button" key={kind} onClick={() => { setNewMenuOpen(false); create(kind); }}>{`New ${kind}`}</button>)}</div>}</div></div></header>
      <main className="page-content"><div className="content-wrap">{page}</div></main>
    </div>
    {recordDialog && <CreateRecordDialog kind={recordDialog} data={data} onClose={() => setRecordDialog(null)} onSave={(values) => saveRecord(recordDialog, values)} />}
    {catalogEdit?.kind === 'product' && <ProductEditDialog key={`product-${catalogEdit.record.id}`} product={catalogEdit.record} onClose={() => setCatalogEdit(null)} onSave={(values) => saveProduct(catalogEdit.record, values)} />}
    {catalogEdit?.kind === 'plan' && <PlanEditDialog key={`plan-${catalogEdit.record.id}`} plan={catalogEdit.record} onClose={() => setCatalogEdit(null)} onSave={(values) => savePlan(catalogEdit.record, values)} />}
    {catalogArchive && <CatalogArchiveDialog key={`${catalogArchive.kind}-${catalogArchive.record.id}`} target={catalogArchive} onClose={() => setCatalogArchive(null)} onArchive={() => archiveCatalogRecord(catalogArchive)} />}
    {paymentInvoice && <PaymentDialog invoice={paymentInvoice} customer={data.customers.find((customer) => customer.id === paymentInvoice.customerId)?.company ?? 'Customer'} onClose={() => setPaymentInvoice(null)} onSave={(values) => savePayment(paymentInvoice, values)} />}
    {review && <ReviewDialog value={review} onClose={() => setReview(null)} />}
    {toast && <div className={`toast ${toastIsError ? 'toast-error' : ''}`} role={toastIsError ? 'alert' : 'status'}><Icon name={toastIsError ? 'close' : 'check'} size={16} />{toast}</div>}
  </div>;
}
