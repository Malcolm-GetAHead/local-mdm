# External Dependencies

Local copies of documentation for key external dependencies used by Local MDM.

## Why Local Copies?

- Offline reference for development and agentic coding
- Annotated with notes relevant to our integration
- Pinned to known versions to avoid drift

## Dependencies

| Dependency | Source | Purpose | Last Pulled |
|---|---|---|---|
| [NanoMDM](nanomdm/) | [micromdm/nanomdm](https://github.com/micromdm/nanomdm) | Apple MDM server & library | 2026-02-06 |
| [NanoDEP](nanodep/) | [micromdm/nanodep](https://github.com/micromdm/nanodep) | Apple DEP API integration | 2026-02-06 |
| [NanoLIB](nanolib/) | [micromdm/nanolib](https://github.com/micromdm/nanolib) | Shared Go library for Nano suite | 2026-02-06 |
| [SCEP](scep/) | [micromdm/scep](https://github.com/micromdm/scep) | SCEP certificate enrollment server | 2026-02-06 |
| [Keycloak + PSSO](keycloak/) | [keycloak.org](https://www.keycloak.org/) / [unioslo](https://github.com/unioslo) | OIDC IdP + macOS Platform SSO | 2026-02-06 |

## Updating

When updating dependency docs, update the "Last Pulled" date above and note any breaking changes or relevant differences from the version we're building against.
