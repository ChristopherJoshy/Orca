import { useEffect, useRef, type FormEvent, type ReactNode } from 'react';
import { Icon, type IconName } from './icons';

export function Button({ children, variant = 'default', size = 'md', className = '', type = 'button', disabled = false, onClick, icon, ariaLabel }: {
  children?: ReactNode; variant?: 'default' | 'primary' | 'quiet' | 'outline' | 'danger'; size?: 'sm' | 'md'; className?: string;
  type?: 'button' | 'submit'; disabled?: boolean; onClick?: () => void; icon?: IconName; ariaLabel?: string;
}) {
  return <button type={type} className={`button button-${variant} button-${size} ${className}`} disabled={disabled} onClick={onClick} aria-label={ariaLabel}>
    {icon && <Icon name={icon} size={16} />}{children}
  </button>;
}

export function Avatar({ initials, size = 'md' }: { initials: string; size?: 'sm' | 'md' | 'lg' }) {
  return <span className={`avatar avatar-${size}`} aria-hidden="true">{initials}</span>;
}

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'green' | 'amber' | 'red' | 'purple' | 'blue' }) {
  return <span className={`badge badge-${tone}`}><span className="badge-dot" />{children}</span>;
}

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <section className={`card ${className}`}>{children}</section>;
}

export function PageHeading({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description?: string; actions?: ReactNode }) {
  return <div className="page-heading">
    <div>{eyebrow && <div className="eyebrow">{eyebrow}</div>}<h1>{title}</h1>{description && <p>{description}</p>}</div>
    {actions && <div className="heading-actions">{actions}</div>}
  </div>;
}

export function EmptyState({ icon, title, description, action }: { icon: IconName; title: string; description: string; action?: ReactNode }) {
  return <div className="empty-state"><div className="empty-icon"><Icon name={icon} size={22} /></div><h3>{title}</h3><p>{description}</p>{action}</div>;
}

export function Dialog({ title, description, onClose, children, wide = false }: { title: string; description?: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const dialogRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    dialogRef.current?.querySelector<HTMLElement>('input,select,textarea,button')?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = [...dialogRef.current.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[href],[tabindex]:not([tabindex="-1"])')];
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [onClose]);
  return <div className="dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <div className={`dialog ${wide ? 'dialog-wide' : ''}`} ref={dialogRef} role="dialog" aria-modal="true" aria-labelledby="dialog-title" aria-describedby={description ? 'dialog-description' : undefined}>
      <div className="dialog-head"><div><h2 id="dialog-title">{title}</h2>{description && <p id="dialog-description">{description}</p>}</div><Button variant="quiet" size="sm" icon="close" ariaLabel="Close dialog" onClick={onClose} /></div>
      {children}
    </div>
  </div>;
}

export function FormActions({ onCancel, submit = 'Save changes', disabled = false }: { onCancel: () => void; submit?: string; disabled?: boolean }) {
  return <div className="form-actions"><Button variant="outline" onClick={onCancel}>Cancel</Button><Button variant="primary" type="submit" disabled={disabled}>{submit}</Button></div>;
}

export function TextField({ label, name, type = 'text', value, onChange, placeholder, error, required = false, min, step }: {
  label: string; name: string; type?: string; value: string; onChange: (value: string) => void; placeholder?: string; error?: string;
  required?: boolean; min?: string; step?: string;
}) {
  const id = `field-${name}`;
  return <label className="field" htmlFor={id}><span>{label}{required && <b className="required">*</b>}</span><input id={id} name={name} type={type} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} required={required} min={min} step={step} aria-invalid={Boolean(error)} aria-describedby={error ? `${id}-error` : undefined} />{error && <small id={`${id}-error`} className="field-error">{error}</small>}</label>;
}

export function SelectField({ label, name, value, onChange, options, required = false }: {
  label: string; name: string; value: string; onChange: (value: string) => void; options: { label: string; value: string }[]; required?: boolean;
}) {
  return <label className="field" htmlFor={`field-${name}`}><span>{label}{required && <b className="required">*</b>}</span><select id={`field-${name}`} name={name} value={value} onChange={(event) => onChange(event.target.value)} required={required}>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>;
}

export function ToggleField({ label, checked, onChange, description }: { label: string; checked: boolean; onChange: (checked: boolean) => void; description?: string }) {
  return <label className="toggle-row"><span><strong>{label}</strong>{description && <small>{description}</small>}</span><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} /><i aria-hidden="true" /></label>;
}

export function Form({ onSubmit, children, className = '' }: { onSubmit: (event: FormEvent<HTMLFormElement>) => void; children: ReactNode; className?: string }) {
  return <form className={`form ${className}`} onSubmit={onSubmit}>{children}</form>;
}

export function IconButton({ icon, label, onClick, className = '' }: { icon: IconName; label: string; onClick: () => void; className?: string }) {
  return <button type="button" className={`icon-button ${className}`} aria-label={label} title={label} onClick={onClick}><Icon name={icon} size={18} /></button>;
}

export function Divider() { return <div className="divider" />; }

export function currencyNote() { return <span className="currency-note">Amounts in INR</span>; }
