# Data exports

## Current implementation

The API creates a full business or filtered ZIP from one store read boundary. PostgreSQL reads use a repeatable-read transaction; the file is generated asynchronously in the API process and retained locally. The archive contains versioned JSON datasets, customer/invoice/payment CSV files, printable invoice HTML, a manifest, record counts, and SHA-256 checksums. JSON values remain unchanged; CSV cells beginning with formula/control prefixes are escaped.

Filters support resource/dataset selection, one or many IDs, status, text search, date bounds, and resource-appropriate customer/invoice/subscription/product/plan/discount selectors. Related records are included to preserve useful references without expanding into unrelated sibling transactions. The archive includes business, catalog, tax, discount, quotation/template, subscription, invoice/snapshot, payment/allocation, journal, audit, and approval data. Authentication and provider secrets are excluded.

Downloads require an authenticated user in the same business and are denied after the recorded 24-hour expiry. Customer requests are scoped to the customer's own portal records. Admin and staff currently share business-level export scope; a finer staff permission model remains open. Files are private on local disk with restrictive directory/file permissions.

## Remaining work

Export jobs and files are not durable across process restarts or host changes. There is no background worker or periodic archive janitor; expiry is enforced by the download handler, and physical cleanup after expiry is still being completed. Store snapshot consistency is established, but generation is not crash-safe. The current API does not provide an import path. Printable invoices are standalone HTML rather than PDF.

## Archive and privacy contract

Keep the archive schema versioned. The manifest records business ID, generated time, requested filters, dataset counts, and checksums. JSON preserves original values. CSV neutralizes formula prefixes (`=`, `+`, `-`, `@`, tab, or carriage return) so spreadsheet programs do not interpret an exported cell as a formula.

Never export password hashes, sessions, reset/verification tokens, OAuth bearer tokens, provider secrets, or payment-card data. Serve archives only through authorization-checked endpoints; never expose public object URLs. Retain files for no longer than 24 hours and delete the private archive after expiry. Importing archives remains out of scope.

## Verification

Focused tests cover filtered selection, relationship closure, tenant isolation, empty ID sets, CSV formula/control-prefix escaping, escaped printable HTML, and private file permissions. The isolated PostgreSQL flow covers ZIP generation/download and clean-schema migrations. Long-running job recovery and automated expiry cleanup are not covered yet.
