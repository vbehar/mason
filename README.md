# Mason

**WORK IN PROGRESS**

This project is under active development and not yet ready for production use.

## Overview

**Mason** is a build tool designed for both CI/CD pipelines and local development. It combines the simplicity of [Maven](https://maven.apache.org/)'s phased UX with the declarative power of Kubernetes-style configuration, all running on top of containerized, language-agnostic [Dagger](https://dagger.io/) modules.

Inspired by the craft of masonry, Mason lets you build complex software systems from simple, declarative building blocks.

## Core Concepts

### Blueprint

A **blueprint** is a collection of declarative build definitions written in YAML or JSON. It describes the expected outputs (binaries, container images, etc.), and how to produce them. Just like architectural blueprints, it's the plan from which everything is built.

### Brick

A **brick** is a single unit of work within a blueprint. Each brick defines:

* A `kind` (e.g. GoBinary, OCIImage)
* A reference to the module that can process it (`moduleRef`)
* `metadata` for name and labels
* A `spec` that describes the expected state or build parameters

Example:

```yaml
kind: GoBinary
moduleRef: github.com/example/go-module.git@v1.0.0
metadata:
  name: my-binary
spec:
  os: linux
  arch: amd64
  output:
    daggerFileName: binary_linux_amd64
    hostFilePath: bin/binary-linux-amd64
```

### Module

A **module** is a Dagger-based implementation that defines how to process one or more kinds of bricks. Modules are language-agnostic and reusable. They:

* Define brick schemas (what's allowed in `spec`)
* Receive the blueprint
* **Render a plan**: a Dagger script that Mason will execute

## Why Mason?

* **Simple CLI UX**: A consistent set of commands (`mason package`, etc.)
* **Modular and Extensible**: Add new kinds of bricks via modules
* **Declarative and Reproducible**: YAML-based configuration with a clean spec
* **Safe and Sandboxed**: All operations run in containers using Dagger

## Status

This is an initial commit. A comprehensive README with installation instructions, examples, and detailed documentation will be added in future updates.
