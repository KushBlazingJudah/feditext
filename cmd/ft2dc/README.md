# ft2dc

Feditext to Discord webhook gateway.
Quick 'n dirty, more of a proof of concept than anything.

## Usage

- Set the environment variable `WEBHOOK_URL` to the webhook URL given by Discord.
- Start the daemon: `go run ./cmd/ft2dc -addr localhost:8081`
- Configure Feditext

In `./feditext.config`:

```
hook web http://localhost:8081
```
