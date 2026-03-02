# ktool

ktool is a CLI tool designed to simplify Kubernetes management tasks. It provides a focused set of commands to interact with your clusters efficiently.

## Installation

### From Source

```bash
go install github.com/aureliomalheiros/ktool@latest
```

## Usage

### Context Management (ctx)

The `ctx` command allows you to view and switch between Kubernetes contexts.

#### List Contexts

To list all available contexts in your kubeconfig:

```bash
ktool ctx
```

This will display the current context marked with an asterisk (*).

#### Switch Context

To switch to a specific context:

```bash
ktool ctx <context_name>
```

#### Interactive Mode

If `fzf` is installed on your system, running `ktool ctx` without arguments will open an interactive fuzzy search menu. You can filter and select the context you want to switch to.

1. Run `ktool ctx`
2. Type to filter contexts
3. Press Enter to select and switch

## Roadmap / Next Features

- [ ] **logs**: Advanced log viewing with tailing and filtering (similar to stern)
- [ ] **dns**: Tools for debugging DNS issues within the cluster
- [ ] **resources**: List Kubernetes resources (pods, services, deployments, etc.)
- [ ] **edit**: Edit resources directly from the CLI
- [ ] **ns**: Namespace switching and management
