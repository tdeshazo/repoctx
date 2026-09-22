# M6-01 active-development contract evidence

M6-01 is complete at the source state containing this report.

Dogfooding `repoctx discover` against compatibility and version terms exposed a
stale `docs/IR.md` section: it described v1alpha3 as current even though the
compiler writes v1alpha4. It also showed that provider versions were emitted in
grounding metadata but were private constants, and that the public Go API had no
inspectable contract identifier.

The resulting change:

- defines a current-contract-only policy, closed-field rules, and rebuild
  requirements in `docs/COMPATIBILITY.md`, with no compatibility window;
- adds `repoctx.go-api/v1alpha1`, exports the two existing provider identities
  without changing their values, and reports all of them in build information;
- advances build information to `repoctx.build/v1alpha2` because its closed JSON
  shape gained fields; and
- replaces stale IR guidance with a link to the central policy; and
- restricts the model-neutral adapter to the current context contract.

`pkg/compat` tests pin every published identifier, while CLI tests pin the new
build-information projection. Full Go tests, vet, Python tests, schema checks,
and deterministic artifact-catalog checks are the reproducible verification for
this item. M6-02 covers current contracts and rejection of stale versions; it
does not preserve historical readers.
