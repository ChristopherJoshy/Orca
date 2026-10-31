# 0013 — Complete customer pause and resume controls

## Result

- Customer portal now offers a direct resume action for an owned paused subscription when the plan allows pause, alongside the existing pause action.
- Renamed “Request pause” to “Pause subscription” because the operation is applied immediately by the API.
- Added HTTP coverage for a customer pausing then resuming an owned subscription. The test checks that the saved pause interval opens and closes.
- Updated the acceptance matrix and local test journey.

## Verification

- Focused customer self-service API test — passed.
- `make verify` — passed: Go tests, race detector, `go vet`, TypeScript, lint alias, and Vite production build.
- `git diff --check` — passed.

## Known limits

- Customer self-service is limited to pause and resume. Full lifecycle policy coverage remains open.
