# 0020 — Visual README and full screenshot gallery

## Result

- Replaced the text-heavy project README with a visual, workflow-led story: an invoice question, the three core product ideas, a feature map, real screenshots, a Mermaid architecture diagram, local setup, verification commands, and honest remaining limits.
- Added a custom SVG hero banner and 20 optimized WebP screenshots covering every workspace page, the customer portal, and mobile views at 390×844.
- Added `docs/screenshots.md` with grouped galleries for the daily workspace, selling, billing and collection, automation, administration, customer experience, and mobile views. Linked it from the README and the documentation index.
- Kept the MVP status explicit: local hackathon build, no production deployment, no live-provider proof, and no claim that open acceptance work is finished.
- Captured from a disposable local account with illustrative records, then deleted the businesses, users, memberships, billing records, payments, export job, and audit events. No seed data or fallback was left in the product.
- Updated `docs/history.md` and the screenshots gallery with the capture and cleanup record.

## Verification

- All local README, gallery, banner, and documentation paths resolve.
- README and gallery heading anchors resolve.
- `git diff --check` passed.
- Screenshots inspected at desktop and mobile widths; the gallery uses the optimized assets rather than the source PNGs.
- `make verify` passed: Go tests, race detector, `go vet`, TypeScript typecheck, lint alias, and production build.

## Known limits

- A few captures show existing development UI copy defects, notably label concatenation on Invoice Truth and an export status rendering issue. They are visible in the gallery and recorded as follow-up work, not described as polished behavior.
- The screenshots use illustrative local records from a temporary workspace; they are not a customer dataset or a demo mode.
