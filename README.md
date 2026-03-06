# ktool

ktool is a CLI tool designed to simplify Kubernetes management tasks. It provides a focused set of commands to interact with your clusters efficiently.

## Requirements

- Go 1.25 or later (for building from source)
- A valid kubeconfig file (`~/.kube/config` or set via `KUBECONFIG`)
- [fzf](https://github.com/junegunn/fzf) (optional, enables interactive selection)

## Installation

### Using go install

```bash
go install github.com/aureliomalheiros/ktool@latest
```

### From source

```bash
git clone https://github.com/aureliomalheiros/ktool.git
cd ktool
go build -o ktool .
```

Then move the binary to a directory in your `PATH`:

```bash
mv ktool /usr/local/bin/
```

### From releases

Download the pre-built binary for your platform from the [releases page](https://github.com/aureliomalheiros/ktool/releases), then move it to a directory in your `PATH`.

## Usage

```
ktool [command]
```

### Context management (ctx)

The `ctx` command lets you list and switch between Kubernetes contexts.

**List all contexts:**

```bash
ktool ctx
```

Output example:

```
CURRENT   NAME           CLUSTER        AUTHINFO   NAMESPACE
*         production     prod-cluster   admin      default
          staging        stg-cluster    admin      default
```

**Switch to a context:**

```bash
ktool ctx <context-name>
```

**Interactive mode (requires fzf):**

If `fzf` is installed, running `ktool ctx` without arguments opens an interactive fuzzy search menu:

1. Run `ktool ctx`
2. Type to filter contexts
3. Press `Enter` to switch

### Namespace management (ns)

The `ns` command lets you list and switch namespaces within the current context. It queries the Kubernetes API, so it reflects the actual namespaces available in the cluster.

**List all namespaces:**

```bash
ktool ns
```

Output example:

```
CURRENT   NAME
          default
          kube-system
*         production
          staging
```

**Switch to a namespace:**

```bash
ktool ns <namespace-name>
```

**Interactive mode (requires fzf):**

If `fzf` is installed, running `ktool ns` without arguments opens an interactive fuzzy search menu:

1. Run `ktool ns`
2. Type to filter namespaces
3. Press `Enter` to switch

The selected namespace is saved into the current context of your kubeconfig, so subsequent `kubectl` commands will target it automatically.

## Roadmap

- [ ] **logs**: Advanced log viewing with tailing and filtering (similar to stern)
- [ ] **dns**: Tools for debugging DNS issues within the cluster
- [ ] **resources**: List Kubernetes resources (pods, services, deployments, etc.)
- [ ] **edit**: Edit resources directly from the CLI
