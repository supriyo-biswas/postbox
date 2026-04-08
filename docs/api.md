# Postbox API Reference

Postbox exposes an HTTP API for reading inboxes, inspecting messages, and downloading attachments. The API is designed to be compatible with the Mailtrap Sandbox API and the inbox, email/message and attachment APIs are supported.

## Introduction

Postbox authenticates API requests with an inbox API key. The key is printed when you create an inbox from the CLI, and the default inbox uses `postbox-default` as the SMTP username, password, and API key. See the [README](../README.md) for instructions on creating inboxes.

The `{inbox}` path segment can be either the inbox numeric ID or the inbox name.

Pass the API key on each request as a header or as a query string parameter. You can pass the API key using any one of the following methods:

```bash
curl -sS "http://localhost:8080/api/v1/inboxes/1" \
  -H "Api-Token: postbox-default"

curl -sS "http://localhost:8080/api/v1/inboxes/1" \
  -H "Authorization: Bearer postbox-default"

curl -sS "http://localhost:8080/api/v1/inboxes/1" \
  -H "Authorization: Token postbox-default"

curl -sS "http://localhost:8080/api/v1/inboxes/1?api_token=postbox-default"
```

Unless otherwise noted, successful responses are JSON. Body and download endpoints return the stored content directly with the MIME type saved in Postbox.

Any endpoint can also return `500 Internal Server Error` if Postbox encounters a database, serialization, or other internal failure.

## Inbox APIs

### 1. Get inbox details

`GET /api/v1/inboxes/{inbox}`

Returns metadata for the inbox, including counts and the timestamp of the latest message.

200 response:

```json
{
  "id": 1,
  "name": "postbox-default",
  "username": "postbox-default",
  "status": "active",
  "email_username": "postbox-default",
  "email_username_enabled": true,
  "sent_messages_count": 12,
  "emails_count": 12,
  "emails_unread_count": 3,
  "last_message_sent_at": "2026-04-08T12:34:56.000Z"
}
```

Notes:

- `last_message_sent_at` is `null` when the inbox has no messages.

4xx conditions:

- `400 Bad Request` if the API key is missing or malformed, or if the inbox identifier is missing from the route.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox does not exist.

### 2. Delete all messages in an inbox

`PATCH /api/v1/inboxes/{inbox}/clean`

Deletes every message in the inbox and returns the updated inbox metadata.

200 response:

```json
{
  "id": 1,
  "name": "postbox-default",
  "username": "postbox-default",
  "status": "active",
  "email_username": "postbox-default",
  "email_username_enabled": true,
  "sent_messages_count": 0,
  "emails_count": 0,
  "emails_unread_count": 0,
  "last_message_sent_at": null
}
```

4xx conditions:

- `400 Bad Request` if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox does not exist.

### 3. Mark all messages as read

`PATCH /api/v1/inboxes/{inbox}/all_read`

Sets `is_read` to `true` for every message in the inbox and returns the updated inbox metadata.

200 response:

```json
{
  "id": 1,
  "name": "postbox-default",
  "username": "postbox-default",
  "status": "active",
  "email_username": "postbox-default",
  "email_username_enabled": true,
  "sent_messages_count": 12,
  "emails_count": 12,
  "emails_unread_count": 0,
  "last_message_sent_at": "2026-04-08T12:34:56.000Z"
}
```

Notes:

- The inbox response does not include per-message state; it only reflects aggregate counts.

4xx conditions:

- `400 Bad Request` if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox does not exist.

### 4. List inbox messages

`GET /api/v1/inboxes/{inbox}/messages`

Returns a paginated list of messages in the inbox.

Query parameters:

- `page` optional, 1-based page number. Defaults to the first page.
- `size` optional, number of messages per page. Defaults to `30`.
- `search` optional, filters messages by subject.

200 response:

```json
[
  {
    "id": 100,
    "inbox_id": 1,
    "subject": "Welcome",
    "sent_at": "2026-04-08T12:34:56.000Z",
    "from_email": "sender@example.com",
    "from_name": "Sender",
    "to_email": "user@example.com",
    "to_name": "User",
    "email_size": 2048,
    "is_read": false,
    "created_at": "2026-04-08T12:34:56.000Z",
    "updated_at": "2026-04-08T12:34:56.000Z",
    "html_body_size": 1024,
    "text_body_size": 512,
    "human_size": "2.0 kB",
    "smtp_information": {
      "ok": true,
      "data": {
        "mail_from_addr": "sender@example.com",
        "client_ip": "127.0.0.1"
      }
    },
    "addresses": {
      "from": [
        {
          "name": "Sender",
          "address": "sender@example.com"
        }
      ],
      "to": [
        {
          "name": "User",
          "address": "user@example.com"
        }
      ],
      "cc": [],
      "bcc": []
    }
  }
]
```

Notes:

- `smtp_information.ok` is always `true` for stored messages.
- `smtp_information.data.mail_from_addr` is the SMTP envelope sender.
- `smtp_information.data.client_ip` is the client IP recorded when the message was received.
- `addresses` groups recipients by `from`, `to`, `cc`, and `bcc`.
- `from_email`, `from_name`, `to_email`, and `to_name` are nullable fields.

4xx conditions:

- `400 Bad Request` if `page` is less than 1 or `size` is less than 2, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox does not exist.

## Message APIs

### 5. Get a message

`GET /api/v1/inboxes/{inbox}/messages/{message}`

Returns the full message metadata, including parsed recipient lists and SMTP information.

200 response:

```json
{
  "id": 100,
  "inbox_id": 1,
  "subject": "Welcome",
  "sent_at": "2026-04-08T12:34:56.000Z",
  "from_email": "sender@example.com",
  "from_name": "Sender",
  "to_email": "user@example.com",
  "to_name": "User",
  "email_size": 2048,
  "is_read": false,
  "created_at": "2026-04-08T12:34:56.000Z",
  "updated_at": "2026-04-08T12:34:56.000Z",
  "html_body_size": 1024,
  "text_body_size": 512,
  "human_size": "2.0 kB",
  "smtp_information": {
    "ok": true,
    "data": {
      "mail_from_addr": "sender@example.com",
      "client_ip": "127.0.0.1"
    }
  },
  "addresses": {
    "from": [
      {
        "name": "Sender",
        "address": "sender@example.com"
      }
    ],
    "to": [
      {
        "name": "User",
        "address": "user@example.com"
      }
    ],
    "cc": [],
    "bcc": []
  }
}
```

Notes:

- `smtp_information.ok` is always `true` for stored messages.
- `addresses` groups recipients by `from`, `to`, `cc`, and `bcc`.
- `from_email`, `from_name`, `to_email`, and `to_name` are nullable fields.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist.

### 6. Update a message

`PATCH /api/v1/inboxes/{inbox}/messages/{message}`

Updates the message read state.

Request body:

```json
{
  "message": {
    "is_read": true
  }
}
```

200 response:

```json
{
  "id": 100,
  "inbox_id": 1,
  "subject": "Welcome",
  "sent_at": "2026-04-08T12:34:56.000Z",
  "from_email": "sender@example.com",
  "from_name": "Sender",
  "to_email": "user@example.com",
  "to_name": "User",
  "email_size": 2048,
  "is_read": true,
  "created_at": "2026-04-08T12:34:56.000Z",
  "updated_at": "2026-04-08T12:34:56.000Z",
  "html_body_size": 1024,
  "text_body_size": 512,
  "human_size": "2.0 kB",
  "smtp_information": {
    "ok": true,
    "data": {
      "mail_from_addr": "sender@example.com",
      "client_ip": "127.0.0.1"
    }
  },
  "addresses": {
    "from": [
      {
        "name": "Sender",
        "address": "sender@example.com"
      }
    ],
    "to": [
      {
        "name": "User",
        "address": "user@example.com"
      }
    ],
    "cc": [],
    "bcc": []
  }
}
```

Notes:

- Only `message.is_read` is read from the request body.
- The response is the updated message record after the save completes.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, if the JSON body cannot be decoded, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist.

### 7. Delete a message

`DELETE /api/v1/inboxes/{inbox}/messages/{message}`

Deletes the message and returns the message data as it existed before deletion.

200 response:

```json
{
  "id": 100,
  "inbox_id": 1,
  "subject": "Welcome",
  "sent_at": "2026-04-08T12:34:56.000Z",
  "from_email": "sender@example.com",
  "from_name": "Sender",
  "to_email": "user@example.com",
  "to_name": "User",
  "email_size": 2048,
  "is_read": false,
  "created_at": "2026-04-08T12:34:56.000Z",
  "updated_at": "2026-04-08T12:34:56.000Z",
  "html_body_size": 1024,
  "text_body_size": 512,
  "human_size": "2.0 kB",
  "smtp_information": {
    "ok": true,
    "data": {
      "mail_from_addr": "sender@example.com",
      "client_ip": "127.0.0.1"
    }
  },
  "addresses": {
    "from": [
      {
        "name": "Sender",
        "address": "sender@example.com"
      }
    ],
    "to": [
      {
        "name": "User",
        "address": "user@example.com"
      }
    ],
    "cc": [],
    "bcc": []
  }
}
```

Notes:

- The response is built before the record is removed from the database.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist.

### 8. Get message headers

`GET /api/v1/inboxes/{inbox}/messages/{message}/headers`

Returns the message headers. Since a header may be repeated, the `headers` field contains the first value for each header, while the `multi_headers` field contains all values for each header as an array. `headers` is kept for compatibility with the Mailtrap API.

200 response:

```json
{
  "headers": {
    "Subject": "Welcome",
    "From": "Sender <sender@example.com>",
    "To": "User <user@example.com>"
  },
  "multi_headers": {
    "Subject": ["Welcome"],
    "From": ["Sender <sender@example.com>"],
    "To": ["User <user@example.com>"]
  }
}
```

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist.

### 9. Get the plain text body

`GET /api/v1/inboxes/{inbox}/messages/{message}/body.txt`

Returns the plain text body stored for the message.

200 response:

```text
Hello from Postbox.
```

Notes:

- `Content-Type` is the MIME type saved for the text body.
- The response body is the raw text content.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist, or if the text body was not stored.

### 10. Get the sanitized HTML body

`GET /api/v1/inboxes/{inbox}/messages/{message}/body.html`

Returns the HTML body after sanitization. It should be safe to render this HTML in a browser since unsafe tags and attributes are removed.

200 response:

```html
<html>
  <body>
    <p>Hello from Postbox.</p>
  </body>
</html>
```

Notes:

- `Content-Type` is the MIME type saved for the HTML body.
- The response body is sanitized before it is written to the client.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist, or if the HTML body was not stored.

### 11. Get the raw HTML body

`GET /api/v1/inboxes/{inbox}/messages/{message}/body.htmlsource`

Returns the raw, unsanitized HTML body as stored. It is not safe to render this HTML in a browser as it may contain scripts or other unsafe content.

200 response:

```html
<html>
  <body>
    <script>alert("x")</script>
    <p>Hello from Postbox.</p>
  </body>
</html>
```

Notes:

- `Content-Type` is the MIME type saved for the HTML body.
- The response body is the original HTML content.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist, or if the HTML body was not stored.

### 12. Get the raw email source in EML format

`GET /api/v1/inboxes/{inbox}/messages/{message}/body.eml`

Returns the raw email source as stored.

200 response:

```text
From: Sender <sender@example.com>
To: User <user@example.com>
Subject: Welcome

Hello from Postbox.
```

Notes:

- `Content-Type` is the MIME type saved for the raw source.
- The response body is the original raw email content.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist, or if the raw source was not stored.

### 13. Get the raw email source alias

`GET /api/v1/inboxes/{inbox}/messages/{message}/body.raw`

Alias for `body.eml`. The response and error behavior are the same.

200 response:

```text
From: Sender <sender@example.com>
To: User <user@example.com>
Subject: Welcome

Hello from Postbox.
```

Notes:

- `Content-Type` is the MIME type saved for the raw source.
- The response body is the original raw email content.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist, or if the raw source was not stored.

### 14. List message attachments

`GET /api/v1/inboxes/{inbox}/messages/{message}/attachments`

Returns the attachments and inline parts associated with the message.

200 response:

```json
[
  {
    "id": 200,
    "message_id": 100,
    "filename": "invoice.pdf",
    "attachment_type": "attachment",
    "content_type": "application/pdf",
    "content_id": null,
    "attachment_size": 4096,
    "created_at": "2026-04-08T12:34:56.000Z",
    "updated_at": "2026-04-08T12:34:56.000Z",
    "attachment_human_size": "4.0 kB"
  },
  {
    "id": 201,
    "message_id": 100,
    "filename": null,
    "attachment_type": "inline",
    "content_type": "image/png",
    "content_id": null,
    "attachment_size": 1024,
    "created_at": "2026-04-08T12:34:56.000Z",
    "updated_at": "2026-04-08T12:34:56.000Z",
    "attachment_human_size": "1.0 kB"
  }
]
```

Notes:

- `attachment_type` is either `attachment` or `inline`.
- `filename` is `null` for inline parts.
- `content_id` is returned but is currently `null`.
- The timestamp fields are derived from the parent message timestamps.

4xx conditions:

- `400 Bad Request` if the message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox or message does not exist.

### 15. Get attachment details

`GET /api/v1/inboxes/{inbox}/messages/{message}/attachments/{attachment}`

Returns metadata for a single attachment or inline part.

200 response:

```json
{
  "id": 200,
  "message_id": 100,
  "filename": "invoice.pdf",
  "attachment_type": "attachment",
  "content_type": "application/pdf",
  "content_id": null,
  "attachment_size": 4096,
  "created_at": "2026-04-08T12:34:56.000Z",
  "updated_at": "2026-04-08T12:34:56.000Z",
  "attachment_human_size": "4.0 kB"
}
```

Notes:

- `attachment_type` is either `attachment` or `inline`.
- `filename` is `null` for inline parts.
- `content_id` is returned but is currently `null`.
- The timestamp fields are derived from the parent message timestamps.

4xx conditions:

- `400 Bad Request` if the attachment id or message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox, message, or attachment does not exist.

### 16. Download an attachment

`GET /api/v1/inboxes/{inbox}/messages/{message}/attachments/{attachment}/download`

Downloads the attachment payload.

200 response:

```http
Content-Type: application/pdf
Content-Disposition: attachment; filename="invoice.pdf"

<binary attachment content>
```

Notes:

- `Content-Type` is the attachment MIME type.
- `Content-Disposition` is set to `attachment; filename="<filename>"`.
- The response body is the binary attachment content.

4xx conditions:

- `400 Bad Request` if the attachment id or message id is not a valid integer, or if the API key is missing or malformed.
- `401 Unauthorized` if the API key does not match the inbox.
- `404 Not Found` if the inbox, message, or attachment does not exist.

## Mailtrap Compatibility

The v2 API exists for Mailtrap compatibility. It uses the same handlers as v1, but the account path segment is present so Mailtrap-compatible clients can keep their expected URL shape. Because Postbox is local and does not have real user accounts, any account number works.

Example v2 request:

```bash
curl -sS "http://localhost:8080/api/accounts/123/inboxes/1/messages" \
  -H "Api-Token: postbox-default"
```

The v1 and v2 APIs behave the same for the endpoints documented above.
