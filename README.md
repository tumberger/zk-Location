# Zero-Knowledge Proof of Location

This is an academic, unaudited PoC implementation of the ZKLP protocol, assompanied by an IEEE-754 compliant floating-point implementation (in SNARKs, with constraint system agnostic optimizations) for Float32 and Float64.

## Usage

Initialize h3 submodule.

```
git submodule init
git submodule update
```

Execute floating-point tests. Test cases are generated from [Berkeley TestFloat](https://github.com/ucb-bar/berkeley-testfloat-3) to ensure IEEE-754 compliance.

```
cd float
go test -test.v
```

Execute ZKLP tests. Test cases are generate for all resolutions, with logarithmic steps towards the hexagon boundary.

```
cd loc2index32
go test -test.v
```