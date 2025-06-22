# Quill Protocol Specification

Quill is a modern, decentralized email protocol designed for secure, extensible, and spam-resistant communication between users and email providers.<br>
It replaces legacy email systems (like SMTP,POP3,IMAP) with a structured, back-and-forth JSON protocol over raw TCP connections.

---

## Table of Contents

1. [Overview](#overview)
2. [Packet Structure](#packet-structure)
3. [Signing & Canonicalization](#signing--canonicalization)
4. [Handshake](#handshake)
5. [Authentication (Optional)](#authentication-optional)
6. [Message Transfer](#message-transfer)
7. [Key Management](#key-management)
8. [Anti-Spam Negotiation](#anti-spam-negotiation)
9. [Error Handling](#error-handling)
10. [Security Considerations](#security-considerations)
11. [Packet Framing & Transport](#packet-framing--transport)

---

## Overview

* **Transport**: Raw TCP with **length-prefixed** JSON packets.
* **TLS**: **Required** for all client–server and server–server connections.
* **Federation**: Each provider uses a CA-signed certificate, manages users, public keys, and trust policies.
* **Extensibility**: Supports adding operations (e.g., delete, move) in future versions.
* **Flow**: Handshake → (Auth) → Message Transfer → Key Management → Errors → …

---

## Packet Structure

All packets follow a unified schema:

```json
{
  "type": "PACKET_TYPE",
  "timestamp": "2025-06-22T17:00:00Z",
  "payload": {"packet-specific fields"},
  "signature": "<base64(sig)>",
  "anti_spam": {"optional, e.g. PoW or token"}
}
```

* **type**: Packet name (e.g., `HANDSHAKE`, `SEND_MESSAGE_INIT`).
* **timestamp**: ISO‑8601, ±60 s skew for replay protection.
* **payload**: Holds all packet-specific data; varies per packet type and is versioned to support protocol evolution.
* **signature**: Always present as a string. For full details on when and how signatures are applied (pre-auth omission, client vs. server signing), see [Signing & Canonicalization](#signing--canonicalization)
* **anti\_spam**:  Optional; provides proof-of-work, reputation, or other anti-abuse metadata. See [Anti-Spam Negotiation](#anti-spam-negotiation) for details on how methods are negotiated and enforced.

---

## Signing & Canonicalization

* **Canonical JSON**: Follow RFC 8785 / JCS (sorted keys, no extra whitespace).
* **Signature Input**: `canonical(JSON(payload)) || timestamp` (concatenation of the serialized payload JSON and timestamp string).
* **Algorithm**: Ed25519 (public key tied to `identity`).

### Signature Usage

* **Pre-auth packets** (e.g., `HANDSHAKE`, `HANDSHAKE_ACK`, `FETCH_KEYS`) do not include the signature field; these packets rely on TLS for authenticity.
* **Post-auth packets** (e.g., `SEND_MESSAGE_INIT`, `SEND_MESSAGE_PART`) **must include** a signature:

  * **Empty string** indicates the server will sign on the client's behalf (server-managed keys).
  * **Non-empty** base64 signature indicates the client has signed using its own private key (BYOK).

Servers MUST verify all non-empty signatures against the user's public key and must populate empty-signature packets with a valid signature once the client's identity is authenticated.

---

## Anti-Spam Negotiation

* **Default v1**: `hashcash`.
* `supported_anti_spam` declared in `HANDSHAKE`.
* `required_anti_spam` set per operation in `HANDSHAKE_ACK`.
* Future methods: `challenge_token`, `proof_of_reputation`, etc.

---

## Security Considerations

* **Replay Protection:** Enforce timestamp skew ±60 s; track recent message IDs.
* **Logging:** Record auth failures, signature errors, spam rejections.
* **Forward Secrecy:** Future support via ephemeral key exchange.

---

## Packet Framing & Transport

* **Framing:** 4-byte big-endian length prefix, then JSON.
* **Transport:** Raw TCP with **mandatory TLS**.
* **Reliability:** Handle partial frames, connection drops, session resumption.

---

## Handshake

**Purpose**: Establish a connection and negotiate protocol version, feature capabilities, and anti-spam requirements before any user-specific operations.

* **identity**: Domain or user entity initiating the connection; used for public key discovery and trust checks.
* **encryption**: End-to-end encryption support flag (not available in v1).
* **max\_chunk\_size**: Maximum byte size for each `SEND_MESSAGE_PART`, allowing efficient streaming of large messages.
* **supported\_anti\_spam**: List of anti-spam methods the initiator can perform (default v1: `hashcash`).
### HANDSHAKE  (Client/Server -> Server)

```json
{
  "type": "HANDSHAKE",
  "timestamp": "2025-06-22T17:00:00Z",
  "payload": {
    "protocol": "quill",
    "version": "1.0",
    "identity": "quillmail.com",
    "capabilities": {
      "encryption": false,
      "max_chunk_size": 65536
    },
    "supported_anti_spam": ["hashcash"]
  },
  "signature": "..."
}
```

### HANDSHAKE\_ACK  (Server -> Server/Client)

```json
{
  "type": "HANDSHAKE_ACK",
  "timestamp": "2025-06-22T17:00:01Z",
  "payload": {
    "accepted": true,
    "version": "1.0",
    "required_anti_spam": {
      "SEND_MESSAGE_INIT": { "type": "hashcash", "bits": 22 },
      "FETCH_KEYS": { "type": "none" }
    },
    "capabilities": {
      "encryption": false,
      "max_chunk_size": 65536
    }
  },
  "signature": "..."
}
```

#### Handshake Rejection  (Server -> Server/Client)

```json
{
  "type": "ERROR",
  "timestamp": "2025-06-22T17:00:01Z",
  "payload": {
    "code": "UNSUPPORTED_VERSION",
    "message": "Protocol v2.0 not supported.",
    "context": "HANDSHAKE",
    "temporary": false
  },
  "signature": "..."
}
```

---

## Authentication (Optional)

Servers may require client authentication; protocol defines a generic structure.

### AUTH  (Client -> Server)

```json
{
  "type": "AUTH",
  "timestamp": "2025-06-22T17:10:00Z",
  "payload": {
    "method": "session_token",
    "credentials": { "token": "abc123-session-token" }
  },
  "signature": "..."
}
```

### AUTH\_ACK  (Server -> Client)

```json
{
  "type": "AUTH_ACK",
  "timestamp": "2025-06-22T17:10:01Z",
  "payload": {
    "accepted": true,
    "session": { "expires_in": 3600, "identity": "alice@quillmail.com" }
  },
  "signature": "..."
}
```

---

## Message Transfer

**Purpose**: Transfer email messages in a structured, chunked manner once the client is authenticated.

**Fields in `SEND_MESSAGE.payload`:**

* **to**: List of primary recipient email addresses.
* **cc**: List of carbon-copy recipient addresses.
* **bcc**: List of blind-carbon-copy recipient addresses (hidden from other recipients).
* **subject**: Email subject line.
* **body**: Object containing the message content:

  * `text`: Plain-text content string.
  * `html`: HTML content string.
* **attachments**: Array of objects with attachment metadata:

  * `filename`: Name of the file.
  * `mimetype`: MIME type (e.g., `application/pdf`).
  * `link`: URL or reference for retrieval; all attachments are link-only in v1.
* **options**:
  * `expires_in_seconds`: Time-to-live after which the server may delete or expire the message automatically.
  * `one_time`: Boolean; if `true`, the server **SHOULD** delete the message after a single successful fetch. Note: malicious providers could ignore this and retain messages indefinitely.
  * `thread_id`: Identifier for grouping related messages into a conversation thread.


### SEND\_MESSAGE  (Client/Server -> Server)


```json
{
  "type": "SEND_MESSAGE_INIT",
  "timestamp": "2025-06-22T17:20:00Z",
  "payload": {
    "message_id": "msg-123",
    "from": "bob@quillmail.xyz",
    "to": ["omer@quillmail.xyz"],
    "cc": ["carol@quillmail.xyz", "Dave@quillmail.xyz"],
    "bcc": ["eve@quillmail.xyz"],
    "subject": "Test email example",
    "body": {
      "text": "This is a text message!",
      "html": "<p>Hi <strong>Bob</strong>,<br>See below.</p>"
    },
    "attachments": [
      { "filename": "report.pdf", "mimetype": "application/pdf", "link": "" },
      { "filename": "diagram.png", "mimetype": "image/png", "link": "https://cdn.example.com/diagram.png" }
    ],
    "options": {
      "expires_in_seconds": 7200,
      "one_time": true,
      "thread_id": ""
    }
  },
  "signature": "...",
  "anti_spam": { "type": "hashcash", "resource": "omer@quillmail.xyz", "bits": 22, "nonce": "000abc123", "timestamp": "..." }
}
```

### SEND\_MESSAGE\_ACK  (Server -> Server/Client)

```json
{
  "type": "SEND_MESSAGE_ACK",
  "timestamp": "2025-06-22T17:20:03Z",
  "payload": {
    "status": "OK",
    "message_id": "msg-123",
    "delivered_to": ["omer@quillmail.xyz"]
  },
  "signature": "..."
}
```

---

## FETCH EMAILS

### FETCH\_EMAILS Overview (Client -> Server)

**Purpose**: Ask the server for a paginated list of email metadata (“overview” mode)

```json
{
  "type": "FETCH_EMAILS",
  "timestamp": "2025-06-22T18:00:00Z",
  "payload": {
    "mode": "overview",
    "folder": "inbox",
    "limit": 20,
    "offset": 0,
    "filters": {
        "search": {
            "keywords":       ["project", "deadline"],
            "exact_phrase":    "team meeting",
            "from":           ["alice@…"],
            "to":             ["itamar@…"],
        },
        "flags": {
            "has_attachments": true,
            "is_read": false,
            "is_starred": true,
        },
        "date_range": {
            "after": "2025-06-01T00:00:00Z",
            "before": "2025-06-25T00:00:00Z",
        }
    }
  },
  "signature": "...",
  "anti_spam": { "type": "hashcash", "resource": "omer@quillmail.xyz", "bits": 22, "nonce": "000abc123", "timestamp": "..." }
}
```

### FETCH\_EMAILS Thread (Client -> Server)


```json
{
  "type": "FETCH_EMAILS",
  "timestamp": "2025-06-22T18:00:00Z",
  "payload": {
    "mode": "thread",
    "thread_id": "thread-abc123",
    "limit": 20,
    "offset": 0,
  },
  "signature": "...",
  "anti_spam": { "type": "hashcash", "resource": "omer@quillmail.xyz", "bits": 22, "nonce": "000abc123", "timestamp": "..." }
}
```


---

## Key Management

Providers decide how to manage user key pairs:

* **Server-Managed Keys**: Provider generates and stores private keys (e.g., encrypted in a database) and handles signing on users' behalf.
* **Bring-Your-Own-Key (BYOK)**: Users generate and upload their own key pairs; provider stores only the public key and clients sign messages locally.

These key management strategies are implementation-specific and not enforced by the protocol.

---

### FETCH\_KEYS  (Server -> Server)

```json
{
  "type": "FETCH_KEYS",
  "timestamp": "2025-06-22T17:30:00Z",
  "payload": { "query": "alice@quillmail.com" },
  "signature": "..."
}
```

### KEY\_RESPONSE  (Server -> Server)

```json
{
  "type": "KEY_RESPONSE",
  "timestamp": "2025-06-22T17:30:01Z",
  "payload": {
    "email": "alice@quillmail.com",
    "public_key": "base64-public-key",
    "expires": "2026-01-01T00:00:00Z"
  },
  "signature": "..."
}
```

## Error Handling

All errors use a unified `ERROR` packet:

```json
{
  "type": "ERROR",
  "timestamp": "2025-06-22T17:40:00Z",
  "payload": {
    "code": "RATE_LIMITED",
    "message": "Too many requests.",
    "context": "SEND_MESSAGE_INIT",
    "temporary": true,
    "retry_after": 60
  },
  "signature": "..."
}
```

**Error Code Categories:**

* **Handshake:** `UNSUPPORTED_VERSION`, `MALFORMED_PACKET`, `TLS_REQUIRED`, `SPAM_POLICY_MISMATCH`, `TOO_MANY_CONNECTIONS`, `PROTOCOL_DISABLED`
* **Auth:** `AUTH_REQUIRED`, `INVALID_TOKEN`, `UNSUPPORTED_METHOD`, `TOO_MANY_ATTEMPTS`, `CHALLENGE_FAILED`
* **Message:** `INVALID_SIGNATURE`, `INVALID_RECIPIENTS`, `TOO_LARGE`, `SPAM_DETECTED`, `BLOCKED_DOMAIN`
* **General:** `RATE_LIMITED`, `INVALID_TIMESTAMP`

//add for fetch mail ask of too many mails
---
