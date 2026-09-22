# Projection negotiation contract

`repoctx.projection/v1alpha1` describes a caller-approved context contract and
the bounded projection selected for a target. The closed
[projection schema](projection.schema.json) accepts either the input `Contract`
or output `Projection`; the `Target` input is defined in the schema and passed
separately to `projection.Negotiate`.

The five lanes are intentionally separate: imperative core, task contract,
repository evidence, execution affordances, and verification obligations.
Repository evidence stays in its evidence lane. Capabilities select supported
lanes, limits bound output, and omissions make dropped data explicit.

This package is data-only. It does not read files, run commands or providers,
grant permissions, handle credentials, write vendor files, promote repository
prose into authority, or claim that an execution or verification occurred.
JSON escaping and source maps are negotiated capabilities, not permissions.

Target limits default to 32 KiB and 256 items per lane, with 16 KiB per evidence
item and 512 source maps. Hard per-target ceilings are 1 MiB, 4,096 items per
lane, 1 MiB per evidence item, and 4,096 source maps. Contract validation caps aggregate input
at 8,192 items and 8 MiB of aggregate text. The implementation additionally
validates UTF-8, source spans, duplicate IDs, capability names, and cross-lane
source-map references.
