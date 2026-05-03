> Back to [README](../../../README.md)

# Signal

The Signal channel connects PicoClaw to [Signal](https://signal.org) messenger via [signal-cli](https://github.com/AsamK/signal-cli), which speaks the Signal protocol and exposes a local HTTP API. PicoClaw receives messages via SSE (Server-Sent Events) on `/api/v1/events` and sends them via JSON-RPC on `/api/v1/rpc`. signal-cli runs as a separate process — typically as a sidecar container.

## Configuration

```json
{
  "channel_list": {
    "signal": {
      "enabled": true,
      "type": "signal",
      "allow_from": ["+1234567890", "+0987654321"],
      "group_trigger": {
        "mention_only": true
      },
      "reasoning_channel_id": "",
      "settings": {
        "account": "+1234567890",
        "signal_cli_url": "http://localhost:8080"
      }
    }
  }
}
```

| Field                              | Type     | Required | Description                                                              |
| ---------------------------------- | -------- | -------- | ------------------------------------------------------------------------ |
| `enabled`                          | bool     | Yes      | Whether to enable the Signal channel                                     |
| `type`                             | string   | Yes      | Must be `"signal"`                                                       |
| `allow_from`                       | string[] | No       | Allowlist of senders (phone numbers). See security note below.           |
| `group_trigger`                    | object   | No       | When to respond in group chats (see below)                               |
| `reasoning_channel_id`             | string   | No       | Optional chat ID to receive the LLM's reasoning/thoughts separately      |
| `settings.account`                 | string   | Yes      | Signal phone number in E.164 format (e.g. `+1234567890`)                 |
| `settings.signal_cli_url`          | string   | No       | signal-cli HTTP API URL (default: `http://localhost:8080`)               |

The `enabled`, `type`, `allow_from`, `group_trigger`, and `reasoning_channel_id` fields are common across all channels (defined on the `Channel` wrapper). The Signal-specific fields live under `settings`.

**Security note on `allow_from`:** if empty or omitted, the bot responds to anyone who messages it. PicoClaw logs a `SECURITY` warning on startup in that case. To explicitly acknowledge a public bot, use `["*"]`.

### Multiple instances

The new V3 channel architecture supports multiple instances of the same channel type. To run two Signal accounts side-by-side, give each entry its own key in `channel_list`:

```json
{
  "channel_list": {
    "signal_personal": {
      "enabled": true,
      "type": "signal",
      "allow_from": ["+1111111111"],
      "settings": { "account": "+1111111111", "signal_cli_url": "http://signal-cli-personal:8080" }
    },
    "signal_family": {
      "enabled": true,
      "type": "signal",
      "allow_from": ["+2222222222", "+3333333333"],
      "settings": { "account": "+2222222222", "signal_cli_url": "http://signal-cli-family:8080" }
    }
  }
}
```

Each instance needs its own signal-cli daemon (registered to a different number). The instance key (`signal_personal`, `signal_family`) is used internally as the channel name in logs and identity routing.

### Group Trigger

`group_trigger` controls when PicoClaw responds to messages in group chats. The logic is:

1. If the bot is **@mentioned** → always respond (mention is stripped from the message).
2. If `mention_only: true` and the bot is not mentioned → ignore.
3. If `prefixes: [...]` is set → respond when the message starts with any prefix (prefix is stripped).
4. If `prefixes` are set but none match and the bot is not mentioned → ignore.
5. Otherwise (no `group_trigger` configured) → respond to every group message.

`mention_only` and `prefixes` can be combined — either triggers a response.

| Field          | Type     | Description                                                      |
| -------------- | -------- | ---------------------------------------------------------------- |
| `mention_only` | bool     | Require a bot @mention to respond in groups                      |
| `prefixes`     | string[] | List of prefixes that trigger a response (prefix is stripped)    |

### Environment Variables

The Signal-specific settings support environment variable overrides:

| Variable                              | Maps to                    |
| ------------------------------------- | -------------------------- |
| `PICOCLAW_CHANNELS_SIGNAL_ACCOUNT`    | `settings.account`         |
| `PICOCLAW_CHANNELS_SIGNAL_CLI_URL`    | `settings.signal_cli_url`  |

`enabled`, `allow_from`, `group_trigger`, and `reasoning_channel_id` are common-field overrides on the `Channel` wrapper and must be set via JSON config (no env vars).

## Setup

To use the Signal channel you need three things, in this order:

1. A Signal account registered (or linked) with signal-cli.
2. signal-cli running as an HTTP daemon.
3. PicoClaw's config pointing at that daemon.

Each step can be done either with Docker or with a bare-metal signal-cli install — they share the same data directory layout, so you can mix and match.

### Step 1: Register a Signal account

signal-cli talks to Signal's servers on your behalf and needs to be associated with a phone number — either by **registering** the number directly (you receive an SMS verification code) or by **linking** as a secondary device to an existing Signal install (e.g. your phone).

Registration state lives in the signal-cli data directory (`/var/lib/signal-cli` in the examples below) and persists across restarts. You only do this once per account.

**Using Docker (no host install needed)**

Create a named volume that Step 2 will reuse, then run `register` + `verify` (or `link`) against the same image you'll use as the daemon:

```bash
docker volume create signal-data

# Option A: register a new number — triggers an SMS verification code
docker run --rm -it -v signal-data:/var/lib/signal-cli \
  registry.gitlab.com/packaging/signal-cli/signal-cli-native:latest \
  --config=/var/lib/signal-cli -a +1234567890 register

# ...then enter the code you received via SMS
docker run --rm -it -v signal-data:/var/lib/signal-cli \
  registry.gitlab.com/packaging/signal-cli/signal-cli-native:latest \
  --config=/var/lib/signal-cli -a +1234567890 verify 123456

# Option B: link as a secondary device (scan the printed QR with your phone
# via Settings -> Linked devices)
docker run --rm -it -v signal-data:/var/lib/signal-cli \
  registry.gitlab.com/packaging/signal-cli/signal-cli-native:latest \
  --config=/var/lib/signal-cli link -n "PicoClaw"
```

**Using a bare-metal signal-cli install**

Install signal-cli from [its releases page](https://github.com/AsamK/signal-cli/releases) and follow the [Quickstart](https://github.com/AsamK/signal-cli/wiki/Quickstart) for `register` + `verify`, or `link -n "PicoClaw"`. The default data directory is `~/.local/share/signal-cli/`; override with `--config=/path/to/dir` if you need a specific location.

### Step 2: Run signal-cli as a daemon

Point the daemon at the same data directory you registered into.

**Option A: Docker Compose sidecar (recommended)**

```yaml
services:
  picoclaw:
    # ... your PicoClaw config ...
    depends_on:
      signal-cli:
        condition: service_healthy

  signal-cli:
    image: registry.gitlab.com/packaging/signal-cli/signal-cli-native:latest
    command:
      - "--config=/var/lib/signal-cli"
      - "-a"
      - "+1234567890"
      - "daemon"
      - "--http=0.0.0.0:8080"
      - "--no-receive-stdout"
      - "--receive-mode=on-start"
      - "--send-read-receipts"
    volumes:
      - signal-data:/var/lib/signal-cli
    healthcheck:
      test:
        - "CMD"
        - "bash"
        - "-c"
        - "exec 3<>/dev/tcp/127.0.0.1/8080; printf 'GET /api/v1/check HTTP/1.0\\r\\nHost: localhost\\r\\n\\r\\n' >&3; head -1 <&3 | grep -q '200'"
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 20s

volumes:
  signal-data:
    external: true  # reuse the volume created in Step 1
```

The `registry.gitlab.com/packaging/signal-cli/signal-cli-native:latest` image is a [GraalVM native build](https://packaging.gitlab.io/signal-cli/) — single binary, no JRE, amd64 + arm64 + armv7. Data directory layout is forward-compatible with bare-metal signal-cli installs, so you can switch between Docker and bare-metal later if needed.

**Option B: Bare-metal**

```bash
signal-cli -a +1234567890 daemon \
  --http=localhost:8080 \
  --receive-mode=on-start \
  --send-read-receipts
```

Run under a process supervisor (systemd, supervisord, etc.) so it restarts on failure. Use `--config=/path/to/dir` if your registration data isn't at the default XDG location.

### Step 3: Configure PicoClaw

Add the `channel_list.signal` block from the [Configuration](#configuration) section to your `config.json`, setting:

- `settings.account` — the number you registered (E.164, same as Step 1)
- `settings.signal_cli_url` — where signal-cli is reachable:
  - `http://signal-cli:8080` when PicoClaw and signal-cli share a Docker Compose network (service name)
  - `http://localhost:8080` when signal-cli runs on the host (bare-metal, or Docker with `network_mode: host`)
- `allow_from` — phone numbers allowed to interact with the bot (see security note above)

Start (or restart) PicoClaw. It will open an SSE stream to `/api/v1/events` and begin receiving messages.

## Features

### Supported Message Types

- **Text messages** — full support with markdown-to-Signal formatting conversion.
- **Typing indicators** — shown while the agent is processing (refreshed every 8s).
- **Emoji reactions** — the bot reacts with 👀 to every inbound message and removes the reaction once it replies. PicoClaw is the reference implementation of the `ReactionCapable` interface.
- **Group chats** — configurable trigger modes (see `group_trigger`).
- **Direct messages** — per-sender identity, allowlist filtering.
- **Attachments (receive)** — images (`image/jpeg`, `image/png`, `image/gif`, `image/webp`), audio and voice messages (`audio/mpeg`, `audio/mp3`, `audio/ogg`, `audio/mp4`, `audio/aac`), video (`video/mp4`), and arbitrary files are downloaded from signal-cli and passed to the agent. Voice notes surface as `[voice message]` in the prompt; other attachments surface as `[file: <name>]` or `[image: photo]`. Temp files are cleaned up after processing.

> Sending attachments from PicoClaw back to Signal is **not** implemented yet.

### Message Formatting

PicoClaw converts markdown in outbound messages to Signal's native `textStyle` ranges. The following conversions are applied:

| Markdown              | Signal rendering   |
| --------------------- | ------------------ |
| `**bold**`            | **bold**           |
| `*italic*`            | *italic*           |
| `~~strikethrough~~`   | ~~strikethrough~~  |
| `` `code` ``          | `monospace`        |
| ` ```fenced blocks``` `| `monospace` block  |
| `[text](url)`         | `text (url)`       |
| `# Heading`           | `Heading` (marker stripped) |
| `- item` / `* item`   | `• item`           |
| `> quote`             | `quote` (marker stripped) |

UTF-16 code unit offsets are computed correctly, so emoji and other surrogate-pair characters do not shift subsequent style ranges.

### Message Length

Signal's maximum message length is 6000 Unicode runes. PicoClaw advertises this via the `MessageLengthProvider` interface; longer agent replies are split automatically by the channel manager.

### Identity

Signal senders are identified in canonical form: `signal:+1234567890`. This ID is used for agent routing, allow-list matching, and session keys.

### Reasoning Channel

When `reasoning_channel_id` is set, the LLM's reasoning/thoughts (not the final reply) are sent to that chat — useful for debugging or side-channel observation without cluttering the main conversation.

## Troubleshooting

### Bot not responding

1. Verify signal-cli is running: `curl http://<signal_cli_url>/api/v1/check` should return HTTP 200.
2. Confirm `settings.account` matches the number registered in signal-cli.
3. If `allow_from` is set, verify the sender is included.
4. Check PicoClaw logs for `signal:` entries.

### Group messages not working

- By default, PicoClaw responds to every group message. Configure `group_trigger` to restrict this.
- In `mention_only` mode, the bot must be explicitly @mentioned; verify the @mention resolves to the bot's own phone number in signal-cli's output.
- signal-cli `v0.13.x` has a known bug where the `mentions` array is empty ([AsamK/signal-cli#1940](https://github.com/AsamK/signal-cli/issues/1940)). Use `v0.14.0` or later for reliable `mention_only` behaviour.

### Connection issues

- PicoClaw automatically reconnects to the SSE stream every 5 seconds after a disconnect, so signal-cli can be restarted without restarting PicoClaw.
- When using `depends_on: signal-cli: condition: service_healthy`, Docker Compose waits for signal-cli's healthcheck before starting PicoClaw — the `bash`-based TCP probe in the example is portable and does not require `curl` or `wget` in the signal-cli image.
