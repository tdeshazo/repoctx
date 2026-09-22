# M6-04 release maintenance evidence

M6-04 is complete for the supported Linux amd64 distribution. The repository
now carries a changelog, contributor and security guidance, first- and
third-party license notices, and a maintainer release procedure. The release
check keeps the executable command surface, Usage specification, current schema
identifiers, compatibility policy, vendored skill, dependency notices, and M6
evidence synchronized.

`scripts/release.py build` accepts only a clean Git revision and an external,
new output directory. It builds the native archive, platform-specific wheel,
and sdist; emits SHA-256 checksums and unsigned provenance; and records the
revision, source epoch, material digests, toolchains, platform, and contract
identifiers. Checksums and the statement prove consistency, not publisher
identity. Publication, signing, tagging, and advisory creation remain external
maintainer actions.

The clean revision `9378a64c91143e635be98c0aba30c1ad361ad5f1` produced two
independent release sets with identical `SHA256SUMS`. Each set passed its own Go
module verification, official Go vulnerability scan, CLI/schema/Usage/skill
synchronization checks, and checksum verification. Archive inspection confirmed
that the sdist includes only concise current M6 evidence from the historical
reports directory; raw evaluation transcripts are excluded. A
dogfood-discovered sdist timestamp variance was fixed by normalizing tar and
gzip metadata, with a focused reproducibility regression test.

Reproduce the release checks and build using the commands and declared
prerequisites in [the release procedure](../RELEASING.md). The complete project
suite, race detector, vet, Python tests, deterministic artifact check, and
whitespace check are the final roadmap-close verification; generated release
artifacts remain outside the repository and are not published by this work.
