# Contributing

If you’re interested in contributing but unsure where to start, feel free to open an issue!

This project is in **alpha**. The API surface, internal architecture, and protocol coverage are still evolving.
Breaking changes may occur. The project is intentionally **performance-first**.

---

## Contributions

Contributions are welcome, especially in the following areas:

- Bug fixes
- Performance improvements
- **Protocol implementation and enrichment**
- Benchmarks
- Documentation

For **large changes**, architectural refactors, or new features,
please open an issue for discussion before submitting a PR.

---

## Engine Loop Guidelines (Important)

Some parts of this project run on a high-frequency engine event loop.
Code on this path is extremely performance-sensitive, so a few special
guidelines apply.

### General expectations

- Code must be fast, deterministic, and predictable
- Work should be bounded and completed quickly
- Buffers and objects should be reused where possible

### I/O on the engine loop

- Avoid blocking or unbounded I/O
- Non-blocking, single-attempt socket I/O is OK
- Event-driven readiness handling is preferred
- If an operation may block, spin, or wait, it should be moved off the loop

### Allocations

- Avoid allocations on hot paths where possible
- Reuse buffers and objects
- Be careful with `append`, `make`, closures, and interface conversions

### When in doubt

If you’re unsure whether something is safe to run on the engine loop,
please ask in an issue or PR — happy to help clarify.
---

## Protocol Status

The protocol implementation is not complete yet.

This is expected at the current stage, and expanding protocol coverage
is one of the most valuable ways to contribute.

Protocol-related contributions are very welcome, including:
- New command support
- Encoding / decoding improvements
- Edge cases and error handling

If you’re looking for a good place to start, protocol coverage is an excellent entry point.

As always, protocol changes should preserve the project’s
performance characteristics.

---

## Issues

Bug reports and performance regressions are especially valuable.

When reporting issues, please include:
- Go version
- OS / architecture
- Reproduction steps
- Benchmarks or profiles if available

---

Thanks for contributing!
