# Ledger Lite

Ledger Lite records small-business invoices and refunds.

The default invoice currency is USD unless the caller supplies another ISO code.
Invoice references are normalized before storage so imports can be compared.
Request timeout is 30 seconds for all settlement calls.

See `docs/operations.md` for settlement and audit policy.
