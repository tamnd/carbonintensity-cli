---
title: "Installation"
description: "Install carbonintensity from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/carbonintensity-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `carbonintensity` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/carbonintensity-cli/cmd/carbonintensity@latest
```

That puts `carbonintensity` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/carbonintensity-cli
cd carbonintensity-cli
make build        # produces ./bin/carbonintensity
./bin/carbonintensity version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/carbonintensity:latest --help
```

## Checking the install

```bash
carbonintensity version
```

prints the version and exits.
