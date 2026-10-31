import { Icon } from './icons';

type LandingPageProps = {
  onSignIn: () => void;
  onCreateWorkspace: () => void;
};

export function LandingPage({ onSignIn, onCreateWorkspace }: LandingPageProps) {
  return <main className="landing-page">
    <header className="landing-header">
      <a className="landing-brand" href="/" aria-label="ORCA home">
        <span className="landing-brand-mark" aria-hidden="true"><span /><span /><span /></span>
        <span>orca</span>
      </a>
      <nav className="landing-nav" aria-label="Main navigation">
        <a href="#product">Product</a><a href="#integrity">Billing integrity</a><a href="#copilot">Copilot</a>
      </nav>
      <div className="landing-header-actions">
        <button className="landing-signin" type="button" onClick={onSignIn}>Sign in</button>
        <button className="landing-create" type="button" onClick={onCreateWorkspace}>Create workspace <Icon name="arrow" size={14} /></button>
      </div>
    </header>

    <section className="landing-hero" id="product">
      <div className="landing-hero-copy">
        <div className="landing-eyebrow"><span /> SUBSCRIPTIONS · RECURRING BILLING</div>
        <h1>Billing that<br /><em>shows its work.</em></h1>
        <p>Run plans, subscriptions, and invoices in one place. See the calculation behind every total and the state of every billing cycle.</p>
        <div className="landing-hero-actions">
          <button className="landing-primary" type="button" onClick={onCreateWorkspace}>Create a workspace <Icon name="arrow" size={16} /></button>
          <button className="landing-secondary" type="button" onClick={onSignIn}>Sign in</button>
        </div>
        <div className="landing-hero-note"><Icon name="shield" size={15} /> Billing calculations stay grounded in stored records.</div>
      </div>

      <figure className="landing-visual" aria-label="A subscription billing cycle from plan to invoice and payment">
        <div className="landing-visual-glow" />
        <div className="landing-visual-topline"><span>ONE CONNECTED BILLING CYCLE</span><span>ORCA</span></div>
        <div className="landing-flow">
          <div className="landing-flow-node"><span className="landing-node-icon plan-node"><Icon name="plans" size={19} /></span><span className="landing-node-label">Plan</span><small>Terms &amp; pricing</small></div>
          <span className="landing-flow-link" aria-hidden="true"><i /></span>
          <div className="landing-flow-node"><span className="landing-node-icon subscription-node"><Icon name="subscriptions" size={19} /></span><span className="landing-node-label">Subscription</span><small>Period &amp; quantity</small></div>
          <span className="landing-flow-link" aria-hidden="true"><i /></span>
          <div className="landing-flow-node landing-flow-featured"><span className="landing-node-icon invoice-node"><Icon name="invoices" size={19} /></span><span className="landing-node-label">Invoice</span><small>Stored calculation</small><span className="landing-node-proof"><Icon name="check" size={12} /> Traceable</span></div>
        </div>
        <div className="landing-visual-bottom"><span><i /> Period identity</span><span><i /> Invoice evidence</span><span><i /> Payment balance</span></div>
      </figure>
    </section>

    <section className="landing-principles" id="integrity" aria-label="ORCA product areas">
      <span>BUILT FOR THE WORK THAT MATTERS</span>
      <div><i>Plans &amp; terms</i><b /><i>Invoices</i><b /><i>Collections</i><b /><i>Revenue reports</i></div>
    </section>

    <section className="landing-difference">
      <div className="landing-section-intro">
        <div className="landing-eyebrow"><span /> MADE TO BE INSPECTED</div>
        <h2>A billing month,<br />without the spreadsheet chase.</h2>
        <p>ORCA connects the records that keep recurring revenue moving, then makes the important details visible.</p>
      </div>
      <div className="landing-feature-grid">
        <article className="landing-feature-card"><div className="landing-feature-icon"><Icon name="invoices" size={19} /></div><span className="landing-feature-index">01 / INVOICE TRUTH</span><h3>Every total has a source.</h3><p>Invoices preserve the applied lines, discounts, tax, and calculation hash so a reviewer can follow the arithmetic.</p><a href="#how-it-works">Explore the workflow <Icon name="arrow" size={13} /></a></article>
        <article className="landing-feature-card"><div className="landing-feature-icon"><Icon name="reports" size={19} /></div><span className="landing-feature-index">02 / BILLING INTEGRITY</span><h3>See what needs attention.</h3><p>Follow due billing work, generated invoices, payment balances, and integrity checks from the same records.</p><a href="#how-it-works">Explore the workflow <Icon name="arrow" size={13} /></a></article>
        <article className="landing-feature-card" id="copilot"><div className="landing-feature-icon"><Icon name="spark" size={19} /></div><span className="landing-feature-index">03 / ORCA COPILOT</span><h3>Assistance with review built in.</h3><p>Ask about billing records or prepare supported actions. Consequential changes still go through ORCA review.</p><a href="#how-it-works">Explore the workflow <Icon name="arrow" size={13} /></a></article>
      </div>
    </section>

    <section className="landing-workflow" id="how-it-works">
      <div className="landing-workflow-copy"><div className="landing-eyebrow"><span /> FROM SETUP TO CLOSE</div><h2>One flow.<br />A clear record at each step.</h2><p>Set up a plan, prepare a quotation, activate a subscription, and reconcile each invoice and payment in the same workspace.</p><button type="button" className="landing-text-link" onClick={onCreateWorkspace}>Start with your workspace <Icon name="arrow" size={14} /></button></div>
      <div className="landing-workflow-list">
        <div><span>01</span><p><strong>Set the terms</strong><small>Products, plans, variants, tax, and discounts.</small></p><Icon name="check" size={16} /></div>
        <div><span>02</span><p><strong>Manage the relationship</strong><small>Quotations, subscriptions, pauses, and renewals.</small></p><Icon name="check" size={16} /></div>
        <div><span>03</span><p><strong>Reconcile the cycle</strong><small>Invoice evidence, payment allocation, and reports.</small></p><Icon name="check" size={16} /></div>
      </div>
    </section>

    <footer className="landing-footer"><a className="landing-brand" href="/" aria-label="ORCA home"><span className="landing-brand-mark" aria-hidden="true"><span /><span /><span /></span><span>orca</span></a><span>Subscription management with reliable billing.</span><button type="button" onClick={onSignIn}>Sign in <Icon name="arrow" size={13} /></button></footer>
  </main>;
}
