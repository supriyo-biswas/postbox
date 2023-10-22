# Postbox: Email testing server for developers

Postbox is an email testing server for developers. It acts as a local SMTP server and saves all emails sent to it, and provides a HTTP API for automated testing.

## Features

- Single binary with no dependencies
- Multiple inboxes to segregate emails by project
- Support for HTML emails and attachments
- Support for STARTTLS and HTTPS
- HTTP API to fetch emails and attachments for automated testing
- Drop in replacement for [Mailtrap](https://mailtrap.io) and supports the same API

## Quick start

1. Download the latest binary for your platform (currently linux/amd64, linux/aarch64, darwin/amd64) are supported:
```bash
curl -sSLfo postbox "https://github.com/supriyo-biswas/postbox/releases/download/$(curl -sSL https://api.github.com/repos/supriyo-biswas/postbox/releases | sed -nr 's/.*"tag_name": "(.*)".*/\1/gp' | head -n1)/postbox-$(uname -sm | tr 'A-Z ' 'a-z-')" && chmod +x postbox
```
2. Create your first inbox by providing an inbox name and note down the credentials:
```bash
./postbox inbox create my-inbox
```
3. Start the server:
```bash
./postbox server
```
4. Send an email to the server by configuring your application to use the following SMTP settings:
  - Host: localhost
  - Port: 8025
  - SMTP Username/Password: from step 3
  - SSL/TLS: None
5. Use the API server to fetch inboxes, emails and attachments:
```bash
curl localhost:8080/api/v1/inboxes/1/messages -H "Api-Token: <token>"
```

For details on the API, see the [API documentation](https://api-docs.mailtrap.io/docs/mailtrap-api-docs/5tjdeg9545058-mailtrap-api). The inbox, email/message and attachment APIs are supported.

Postbox is compatible with both v1 and v2 APIs. For the v2 APIs, simply pass in any random number for the account ID, since it is a local service and does not have the concept of user accounts.

## Advanced usage

If you want to configure STARTTLS support for the SMTP server, add HTTPS for the API server, or configure the server to listen on a different port, like this:

```toml
[server.smtp]
    port = 2525 # SMTP port, default is 8025
    key_file = "my-key.pem" # TLS key file, for STARTTLS
    cert_file = "my-cert.pem" # TLS cert file, for STARTTLS
    max_message_bytes = 1000000 # Max size of an email, in bytes (default is 10MB)

[server.http]
    port = 2580 # HTTP port, default is 8080
    key_file = "my-key.pem" # TLS key file, for HTTPS
    cert_file = "my-cert.pem" # TLS cert file, for HTTPS

[database]
    path = "/tmp" # Custom path to postbox's database

[logging]
    filename = "/tmp/postbox.log" # To configure logging to a file, instead of stdout
    max_size = 100 # Max size of the log file, in MB
    max_backups = 10 # Max number of log files to keep
    max_age = 7 # Max age of the log file, in days
```

Place this configuration file in `~/.config/postbox/config.toml` on Linux, or `~/Library/Application Support/postbox/config.toml` on macOS. Alternatively, pass the configuration file in each invocation:

```bash
./postbox server --config /path/to/config.toml
./postbox inbox create my-inbox --config /path/to/config.toml
```
