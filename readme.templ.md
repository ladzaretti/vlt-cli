<!-- omit in toc -->
<p align="center">
  <img src="./assets/gopher_guard.png" alt="Gopher Guard" width="256"/>
</p>

# vlt - A secure command-line tool for managing secrets in your terminal.

[![GitHub release](https://img.shields.io/github/v/release/ladzaretti/vlt-cli)](https://github.com/ladzaretti/vlt-cli/releases)
![status: beta](https://img.shields.io/badge/status-beta-yellow)
![coverage](https://img.shields.io/badge/coverage-{{COVERAGE}}25-yellow)
[![Go Report Card](https://goreportcard.com/badge/github.com/ladzaretti/vlt-cli)](https://goreportcard.com/report/github.com/ladzaretti/vlt-cli)
![license](https://img.shields.io/github/license/ladzaretti/vlt-cli)

`vlt` provides secure, local management of your sensitive information, ensuring your secrets remain encrypted at rest and are only briefly decrypted in memory when accessed.

<!-- omit in toc -->
## Table of Content

- [vlt - A secure command-line tool for managing secrets in your terminal.](#vlt---a-secure-command-line-tool-for-managing-secrets-in-your-terminal)
  - [Supported Platforms](#supported-platforms)
  - [Installation](#installation)
    - [Option 1: Install via curl](#option-1-install-via-curl)
    - [Option 2: Download a release](#option-2-download-a-release)
      - [Optional install script](#optional-install-script)
    - [Option 3: Build from source (requires Go 1.24)](#option-3-build-from-source-requires-go-124)
  - [Design Overview](#design-overview)
    - [vlt - cli client](#vlt---cli-client)
    - [vltd - session manager daemon](#vltd---session-manager-daemon)
  - [Crypto/Security](#cryptosecurity)
  - [Usage](#usage)
  - [Configuration file](#configuration-file)
  - [Examples](#examples)
    - [Tips and Tricks](#tips-and-tricks)
      - [Interactive Secret Selection](#interactive-secret-selection)
      - [Sync to a Git Repository](#sync-to-a-git-repository)

## Supported Platforms

- **OS**: Linux
  - Tested on (`amd64`):
    - Debian 12
    - Fedora 42
- **Arch**: Prebuilt binaries are available for `amd64`, `arm64`, and `386`.

## Installation

### Option 1: Install via curl

```bash
curl -sSL https://raw.githubusercontent.com/ladzaretti/vlt-cli/refs/heads/main/install.sh | bash
```
This script:
- Detects your OS and architecture
- Downloads the latest release from GitHub
- Extracts the archive
- Runs the included install.sh to copy binaries and optionally install the systemd service

### Option 2: Download a release

Visit the [Releases](https://github.com/ladzaretti/vlt-cli/releases) page for a list of available downloads.

#### Optional install script
After downloading and extracting an archive, the `install.sh` script can be used to:

- Copy the `vlt` and `vltd` binaries to `/usr/local/bin`
- Install and enable the `vltd` systemd user service for managing vault sessions

### Option 3: Build from source (requires Go 1.24)

```bash
# Clone and build
git clone https://github.com/ladzaretti/vlt-cli.git
cd vlt-cli
make build-dist

# Optional: run the install script
./dist/install.sh
```
This packs the `vlt` and `vltd` binaries in `./dist/`.

>[!WARNING]
> Installation via `go install` is not supported due to a patched vendored dependency.

## Design Overview
### vlt - cli client
The `vlt` cli manages secrets stored in a vault system composed of two layers:
- `vault_container.sqlite` is the outer SQLite database. It stores metadata and a single encrypted, serialized SQLite instance as a binary blob.
- `vault.sqlite` is a serialized and encrypted inner SQLite database that contains the actual user data.
  - The decrypted `vault.sqlite` is held in the `vlt` process memory only and is never written to disk.

### vltd - session manager daemon
The `vltd` daemon manages derived encryption keys and exposes a Unix socket that `vlt` uses to obtain them. Only `vlt` accesses the database files directly.

```mermaid
graph LR
    subgraph VltFile[".vlt file"]
      subgraph VaultContainer["vault_container.sqlite database"]
          EncryptedVault["vault.sqlite (encrypted serialized database blob)"]
        end
    end

    vlt["vlt (client)"]
    vltd["vltd (daemon)"]
    socket["Unix socket"]

    vlt -->|read/write| VaultContainer
    vlt -->|decrypt + access| EncryptedVault
    vlt -->|request/store session keys| socket --> vltd
```

## Crypto/Security
- **Key Derivation & Auth**: Uses `argon2id` to derive keys from the master password and verify authentication.

- **Encryption**:  
  - Secrets are encrypted with `AES-256-GCM`, using unique nonces for each encrypted value.  
  - The backing `SQLite` database is encrypted at rest and only decrypted into memory after authentication.

- **Memory-Safety**: Secrets are stored in memory only.

## Usage
```console
$ vlt --help
{{USAGE}}
```

## Configuration file

The optional configuration file can be generated using `vlt config generate` command:

```toml
{{CONFIG}}
```

## Examples

These are minimal examples to get you started.  
For detailed usage and more examples, run each subcommand with `--help`.

```shell
# Create a new vault
vlt create

# Import secrets from a file (auto-detects format if compatible, e.g., Firefox or Chromium)
vlt import passwords.csv

# Save a secret interactively
vlt save

# Remove a secret by its name or label
vlt remove foo

# Find secrets with names or labels containing "foo"
vlt find "*foo*"

# List all secrets in the vault
vlt find

# Show a secret by name or label and copy its value to the clipboard
vlt show foo --copy-clipboard

# Show a secret by ID and write its value to a file
vlt show --id 42 --output secret.file

# Use a glob pattern and label filter, print to stdout (unsafe)
vlt show "*foo*" --label "*bar*" --stdout

# Rename a secret by ID
vlt update --id 42 --set-name foo

# Update secret value with a random generated secret
vlt update secret foo --generate

# Rotate the master password
vlt rotate
```


### Tips and Tricks

#### Interactive Secret Selection

```shell
# Use fzf to select a secret interactively and copy its value to the clipboard
vlt login
vlt ls -P | fzf --header-lines=1 | awk '{print $1}' | xargs -r vlt show -c --id
```

#### Sync to a Git Repository
Use the `post-login` and `post-write` hooks to sync the vault with a bare Git repository.

Example setup using fish shell:
```shell
# Bare git repository alias
$ cat .config/fish/alias.fish | grep vault
alias vault_git='/usr/bin/git --git-dir="$HOME/.vltd/" --work-tree="$HOME"'

# Vault hooks configuration
$ cat ~/.vlt.toml | grep -A3 hooks
[hooks]
post_login_cmd=['fish','-c','vault_git pull']
post_write_cmd=['fish','-c',"vault_git add -u && vault_git commit -m \"$(date +'%Y-%m-%d %H:%M:%S')\" && vault_git push"]
```
