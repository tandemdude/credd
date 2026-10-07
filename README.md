# Credd

Credd is a tool to permit injecting secret values into command-line tools. Currently, this is done using environment
variable substitution, but there is scope for extending this to add CLI-argument substitution in the future.

Secrets are never stored by credd itself; they are always read from an external secret manager. Currently
1Password is the only supported secret manager. The CLI tool calls out to `creddserver`, which handles the
secret manager integrations.

> [!NOTE]
> Currently, only MAC OS X is supported. If you are interested in support for Windows / Linux then feel
> free to create an issue.

## Installation

Currently, installation has to be done by building from source. As a prerequisite, you should have golang 1.26 or
higher installed.

**Clone the repository:**

```bash
git clone https://github.com/tandemdude/credd.git && cd credd
```

**Build `credd` and `creddserver`:**

Required prerequisites:
- Install [Taskfile](https://taskfile.dev/)
- Install [SQLc](https://sqlc.dev/)

```bash
task build
```

**Copy binaries to PATH:**

On MAC OS X I just place the binaries in `/usr/local/bin`.

```bash
sudo mv credd /usr/local/bin/credd
sudo mv creddserver /usr/local/bin/creddserver
```

Run `credd` and the help text should now show up.

## Setup

Prior to using the CLI, you should install the background service which handles the database and 1password
integration. This can be done using the helper wizard ``credd init``. You may change any of the config options
presented to you, or choose to accept the defaults.

When asked for your 1Password account, you can enter the name of the account as shown in the top left of your
1Password desktop application.

> [!IMPORTANT]
> The 1Password desktop application is required to be installed for the integration to function.

You should choose `y` when prompted if `creddserver` should run at login, unless running the server some other way,
e.g. through docker. This will install a `launchd` service that will run on user login.

```bash
$ credd init
server address [127.0.0.1:50051]:
1Password account (press enter to skip):
wrote config to ~/.credd/config.toml
Run creddserver automatically at login? [y/N]: y
```

## Usage

To run a command with `credd` substitutions, use the `credd run` command.

```bash
credd run
  --env FOO=op://Vault/Secret/Value # populate env var 'FOO' with the value of the 1Password secret
  -- python3 -c 'import os; print(os.environ["FOO"])'  # command to run
```

An `--env` value may also be a **template**: literal text with one or more `{ref}` placeholders,
where each `ref` is a secret reference such as `op://Vault/Secret/Value`. This is useful for embedding a secret inside
a larger string, such as a database connection URI, without having to assemble it separately.

```bash
credd run
  --env 'DATABASE_URI=postgresql://user:{op://Vault/Secret/Password}@host:5432/dbname'
  -- some-command
```

A value containing no `{...}` placeholders is treated as a single secret reference in its entirety. To include a literal brace in a template, double it (`{{` or `}}`).

To check a secret or print its value without running a command, use `credd secret`:

```bash
credd secret exists op://Vault/Secret/Value  # exit status 0 if the secret exists, 1 if not
credd secret show op://Vault/Secret/Value    # print the secret's value
```

An example usage of this is within your `.claude.json` file, to be able to configure MCP server secrets
outside the hardcoded configuration file. This can be combined with pre-configured env vars which
will correctly be passed through to the child process.

```json
{
  "mcpServers": {
    "Sentry": {
      "type": "stdio",
      "command": "credd",
      "args": [
        "run",
        "--env",
        "SENTRY_ACCESS_TOKEN=op://Private/Sentry/AccessToken",
        "--",
        "npx",
        "@sentry/mcp-server@latest"
      ],
      "env": {
        "SENTRY_HOST": "sentry.example.com"
      }
    }
  }
}
```
