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

### Profiles

A profile is a named collection of env vars, so a set of variables can be reused across commands without
repeating `--env` flags. A profile var can hold either a secret or a plain value:

- a value that is a secret reference (e.g. `op://Vault/Secret/Value`), or a template containing one
  (e.g. `postgresql://user:{op://Vault/DB/Password}@host/db`), is a **secret** and is resolved when the
  command runs
- anything else (e.g. `dev`) is a **plain** value and is used as-is

Profiles never store secret values; only the references are stored.

```bash
credd profile create dev --description "local development against the dev cluster"
credd profile set dev ENV=dev SENTRY_ACCESS_TOKEN=op://Private/Sentry/AccessToken
credd profile show dev       # list vars, their kind, and their (unresolved) values
credd profile validate dev   # check every referenced secret exists, without fetching any values
```

Use `--profile` (or `-p`) with `credd run` to inject a profile's vars. It can be repeated to combine
profiles: later profiles add to earlier ones and override any vars they share. Individual `--env` flags
can still be used alongside profiles, and always take precedence over them.

```bash
credd run --profile base --profile dev --env EXTRA=op://Vault/Secret/Value -- some-command
```

Profiles can be shared as JSON files. An export contains the profile's name, description and vars, with
secret references exported as-is (never their values), so the recipient needs access to the same secrets.
On import, var kinds are re-detected against the importer's configured secret stores.

```bash
credd profile export dev -o dev.json             # or omit -o to write to stdout
credd profile import dev.json                    # keeps the name in the file; fails if it is taken
credd profile import dev.json --name dev-alice   # import under a different name
credd profile import dev.json --overwrite        # replace an existing profile of the same name
credd profile import - < dev.json                # read from stdin
```

The other `credd profile` subcommands are `list` (names and descriptions), `describe <profile> <description>`,
`unset <profile> NAME...` and `delete <profile>`.

### Secrets

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
