# NanoLIB

> Source: [github.com/micromdm/nanolib](https://github.com/micromdm/nanolib)

NanoLIB is a shared Go library of packages used by the "Nano" suite of MDM projects (NanoMDM, NanoDEP, etc.).

## Role in Local MDM

NanoLIB is a transitive dependency — it's pulled in automatically when we import NanoMDM or NanoDEP. It provides common utilities like logging, storage helpers, and other shared infrastructure code.

## Key Packages

See the [Go Reference documentation](https://pkg.go.dev/github.com/micromdm/nanolib) for the full package list. This is a utility library with no standalone binaries or APIs.
