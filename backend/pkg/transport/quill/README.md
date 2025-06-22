# Quill Protocol Specification

Quill is a modern, decentralized email protocol designed for secure, extensible, and spam-resistant communication between users and email providers.<br>
It replaces legacy email systems (like SMTP) with a structured, back-and-forth JSON protocol over raw TCP connections.

---

## Table of Contents

1. [Overview](#overview)
2. [Packet Structure](#packet-structure)
3. [Signing & Canonicalization](#signing--canonicalization)
4. [Handshake](#handshake)
5. [Authentication (Optional)](#authentication-optional)
6. [Message Transfer (Example; Extensible)](#message-transfer-example-extensible)
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
* **payload**: Encapsulates data; versioned and extensible.
* **signature**: Mandatory; Ed25519 over canonical JSON of `payload + timestamp`.
* **anti\_spam**: Optional; negotiated per operation in Handshake.

---

## Signing & Canonicalization

* **Canonical JSON**: Follow RFC 8785 / JCS (sorted keys, no extra whitespace).
* **Signature Input**: `canonical(JSON(payload)) || timestamp`.
* **Algorithm**: Ed25519 (public key tied to `identity`).

---

## Handshake

Negotiates protocol version, capabilities, and anti‑spam policy.

### HANDSHAKE

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

### HANDSHAKE\_ACK

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

#### Handshake Rejection

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

### AUTH

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

### AUTH\_ACK

```json
{
  "type": "AUTH_ACK",
  "timestamp": "2025-06-22T17:10:01Z",
  "payload": {
    "status": "OK",
    "session": { "expires_in": 3600, "identity": "alice@quillmail.com" }
  },
  "signature": "..."
}
```

---

## Message Transfer (Example; Extensible)

**Note**: Future operations like delete/folder management can follow similar patterns.

### SEND\_MESSAGE\_INIT

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
    "attachments": [
      { "filename": "photo.jpg", "mimetype": "image/jpeg", "link": "..." }
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

### SEND\_MESSAGE\_PART (multiple)

```json
{
  "type": "SEND_MESSAGE_PART",
  "timestamp": "2025-06-22T17:20:01Z",
  "payload": {
    "message_id": "msg-123",
    "chunk_index": 0,
    "total_chunks": 2,
    "body": { "type": "text/plain", "value": "This is a text message!" }
  },
  "signature": "..."
}
```

```json
{
  "type": "SEND_MESSAGE_PART",
  "timestamp": "2025-06-22T17:20:02Z",
  "payload": {
    "message_id": "msg-123",
    "chunk_index": 1,
    "total_chunks": 2,
    "body": { "type": "text/html", "value": "<p>Hi <strong>Bob</strong>...</p>" }
  },
  "signature": "..."
}
```

### SEND\_MESSAGE\_ACK

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

## Key Management

Providers decide how to manage user key pairs:

* **Server-Managed Keys**: Provider generates and stores private keys (e.g., encrypted in a database) and handles signing on users' behalf.
* **Bring-Your-Own-Key (BYOK)**: Users generate and upload their own key pairs; provider stores only the public key and clients sign messages locally.

These key management strategies are implementation-specific and not enforced by the protocol.

---

## Key Management

### FETCH\_KEYS

```json
{
  "type": "FETCH_KEYS",
  "timestamp": "2025-06-22T17:30:00Z",
  "payload": { "query": "alice@quillmail.com" },
  "signature": "..."
}
```

### KEY\_RESPONSE

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

---

## Anti-Spam Negotiation

* **Default v1**: `hashcash`.
* `supported_anti_spam` declared in `HANDSHAKE`.
* `required_anti_spam` set per operation in `HANDSHAKE_ACK`.
* Future methods: `challenge_token`, `proof_of_reputation`, etc.

---

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
