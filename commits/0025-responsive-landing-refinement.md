# 0025 — Improve public landing responsiveness and type

## Summary

Refined the public ORCA homepage after owner feedback that its type was too small and its frame too centered. Increased readable text sizes and used a wider desktop composition, with tablet-specific layout changes to keep navigation and product labels visible.

## Changes

- Enlarged body copy, feature cards, billing-cycle labels, buttons, navigation, and workflow text.
- Expanded the max-width of the header, hero, product sections, and footer.
- Stacked the hero and collapsed the header navigation before tablet widths become cramped; wrapped the principle rail on tablets.
- Updated the design contract and recorded the observed layout issue and its prevention.

## Verification

- Frontend typecheck and lint passed; production Vite build passed with the canonical HTTPS origin.
- Browser checks at 2048×1168, 1440×900, 1024×768, 900×900, 834×1112, 768×1024, 390×844, 375×812, and 320×740 found no horizontal overflow.
- Axe reported zero violations and zero incomplete checks. Tablet navigation/principles layout was inspected; wide desktop, tablet, and mobile screenshots were reviewed.
- `git diff --check` passed.

## Limitations

- The post-push Vercel production and live browser checks are recorded in [commit record 0026](0026-live-responsive-landing-release.md).
