---
version: repoctx.frontmatter/v1alpha1
entities:
  - id: maintainers
    kind: owner
    owners: []
    scopes: []
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness: {inputs: []}
  - id: target-state
    kind: document
    owners: [maintainers]
    scopes: [{kind: file, path: docs/target-state.md}]
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness: {inputs: []}
---
# Target state and source-of-truth boundaries

This is the canonical-leaf content selected from the parent advisory bundle.
It remains evidence for retrieval and does not grant runtime authority.
