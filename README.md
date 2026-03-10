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

### Namespace Management (ns)

The `ns` command allows you to view and switch between Kubernetes namespaces.

#### List Namespaces

```bash
ktool ns
```

#### Switch Namespace

```bash
ktool ns <namespace_name>
```

#### Interactive Mode

If `fzf` is installed, running `ktool ns` without arguments opens an interactive fuzzy search menu.

### Log Viewing (logs)

The `logs` command streams logs from pods in the current namespace.

#### List and Select Pod (Interactive)

If `fzf` is installed, running `ktool logs` without arguments opens an interactive pod selection menu:

```bash
ktool logs
```

#### Stream Logs from a Specific Pod

```bash
ktool logs <pod-name>
```

#### Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f, --follow` | false | Follow log output |
| `-c, --container` | | Container name (for multi-container pods) |
| `--tail` | 50 | Lines from end of log (-1 for all) |
| `--since` | | Show logs since duration (e.g. `1h`, `30m`, `5s`) |
| `-n, --namespace` | current | Target namespace |

## Roadmap / Next Features

- [x] **logs**: Advanced log viewing with tailing and filtering (similar to stern)
- [ ] **dns**: Tools for debugging DNS issues within the cluster
- [ ] **resources**: List Kubernetes resources (pods, services, deployments, etc.)
- [ ] **edit**: Edit resources directly from the CLI
