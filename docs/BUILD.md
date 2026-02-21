# Building dctl

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [goreleaser](https://goreleaser.com/) (for cross-platform builds)

## Clone

```shell
git clone https://github.com/mchmarny/dctl.git
cd dctl
```

## Build

Build for your current platform:

```shell
make build
```

The binary is in `./dist`. To install it to `/usr/local/bin`:

```shell
make local
```

## Test

```shell
make test
```

## Full qualification (test + lint + vulnerability scan)

```shell
make qualify
```

## Run locally (development)

```shell
make server
```

## Available Makefile targets

```shell
make help
```
