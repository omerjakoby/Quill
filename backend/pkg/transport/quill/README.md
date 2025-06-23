# Quill Protocol Specification

Quill is a modern, decentralized email protocol designed for secure, extensible, and spam-resistant communication between users and email providers.<br>
It replaces legacy email systems (like SMTP,POP3,IMAP) with a structured, back-and-forth JSON protocol over raw TCP connections.

---

## Table of Contents

1.  [Overview](#overview)
2.  [Packet Structure](#packet-structure)
3.  [Signing & Canonicalization](#signing--canonicalization)
4.  [Anti-Spam Negotiation](#anti-spam-negotiation)
5.  [Security Considerations](#security-considerations)
6.  [Message Attributes](#message-attributes)
7.  [Packet Framing & Transport](#packet-framing--transport)
8.  [Handshake](#handshake)
9.  [Authentication (Optional)](#authentication-optional)
10.  [Sending Emails](#Sending-Emails)
11. [Fetching Emails](#fetching-emails)
12. [Key Management & Federation](#Key-Management-and-Federation)
13. [Error Handling](#error-handling)
14. [Error Codes](#Error-Code-Categories)

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
  "signature": "<base64(sig)>",  // Optional: See Signing section
  "anti_spam": {"optional, e.g. PoW or token"}
}
```

* **type**: Packet name (e.g., `HANDSHAKE`, `SEND_EMAIL`).
* **timestamp**: ISO‑8601, ±60 s skew for replay protection.
* **payload**: Holds all packet-specific data; varies per packet type and is versioned to support protocol evolution.
* **signature**:  Optional. Present only on post-authentication packets. For full details on when and how signatures are applied, see [Signing & Canonicalization](#signing--canonicalization).
* **anti\_spam**:  Optional; provides proof-of-work, reputation, or other anti-abuse metadata. See [Anti-Spam Negotiation](#anti-spam-negotiation) for details on how methods are negotiated and enforced.

---

## Signing & Canonicalization

* **Canonical JSON**: Follow RFC 8785 / JCS (sorted keys, no extra whitespace).
* **Signature Input**: `canonical(JSON(payload)) || timestamp` (concatenation of the serialized payload JSON and timestamp string).
* **Algorithm**: Ed25519 (public key tied to `identity`).

### Signature Usage

* **Pre-auth packets** (e.g., `HANDSHAKE`, `HANDSHAKE_ACK`, `FETCH_KEYS`) do not include the signature field; these packets rely on TLS for authenticity.
* **Post-auth packets** (e.g., `SEND_EMAIL`, `FETCH_EMAIL`) **must include** a signature:

  * **Empty string** indicates the server will sign on the client's behalf (server-managed keys).
  * **Non-empty** base64 signature indicates the client has signed using its own private key (BYOK).

Servers MUST verify all non-empty signatures against the user's public key.<br>
If a client sends a packet with an empty signature, the server (after authenticating the client) MUST generate a valid signature for any subsequent packets it creates to fulfill the request (e.g., when relaying a SEND_EMAIL packet to another server).

---

## Anti-Spam Negotiation

* **Default v1**: `hashcash`.
* `supported_anti_spam` declared in `HANDSHAKE`.
* `required_anti_spam` set per operation in `HANDSHAKE_ACK`.
* Future methods: `challenge_token`, `proof_of_reputation`, etc.

---

### Hashcash

The `hashcash` method requires the client to find a nonce that, when combined with other data, produces a hash with a certain number of leading zero bits. <br> 
This proves that the client has expended computational resources, making bulk spamming expensive.

The anti_spam object for a hashcash proof must contain the following fields:
*   **`type`**: Must be `"hashcash"`.
*   **`resource`**: The unique string identifying the operation being protected. This prevents the replay of the same proof for different actions. The value depends on the operation:
    *   For `SEND_EMAIL`, the resource **MUST** be the `message_id`.
    *   For `FETCH_EMAIL`, the resource **MUST** be the authenticated user's identity (e.g., `alice~quillmail.com`).
*   **`bits`**: The number of leading zero bits required in the hash, as specified by the server in the `HANDSHAKE_ACK`. This determines the difficulty.
*   **`nonce`**: The counter value found by the client that satisfies the proof-of-work challenge.


```json
"anti_spam": {
  "type": "hashcash",
  "resource": "unique-operation-identifier",
  "bits": 22,
  "nonce": "000abc123"
}
```

---

## Security Considerations

* **Replay Protection:** Enforce timestamp skew ±60 s; track recent message IDs.
* **Logging:** Record auth failures, signature errors, spam rejections.
* **Forward Secrecy:** Future support via ephemeral key exchange.
* **BCC Privacy**: Servers MUST strip the bcc field from messages before delivering them to any to or cc recipients. However, the bcc field MUST be preserved when the message is fetched by the original sender.

---

## Message Attributes

Every message in the Quill protocol is defined by a hierarchy of attributes that determine its location, classification, and state. These attributes are managed using the `UPDATE_EMAIL` command.

### 1. Folder (Location)

A **Folder** represents the primary, mutually exclusive location of a message. A message can only be in one folder at a time.

*   `inbox`: The default location for new, incoming messages.
*   `sent`: A copy of messages sent by the user.
*   `archive`: Messages kept but hidden from the main inbox view.
*   `trash`: Messages marked for deletion. Servers typically have a policy to permanently delete items from this folder.
*   `spam`: Messages identified as unsolicited junk mail.

Servers **MUST** support these standard folders.

### 2. Category (Classification)

A **Category** is a classification that provides sub-organization for messages **only within the `inbox` folder**.

*   The receiving server is responsible for automatically classifying incoming messages into a category.
*   If a message is moved from the `inbox` to any other folder (e.g., `trash`, `archive`), its category is cleared by the server.
*   Setting a category is only a valid operation for messages currently located in the `inbox`.

The standard categories are:

*   `inbox`: The default location for new, incoming messages.
*   `sent`: Contains a copy of messages sent by the user.
*   `archive`: For messages that should be kept but hidden from the main inbox view.
*   `trash`: For messages marked for deletion. The server is responsible for its own policy on permanently deleting items from the trash (e.g., after 30 days).
*   `spam`: For messages identified as unsolicited junk mail.

### 3. Flags (State)

**Flags** are independent, boolean states that can be applied to any message, regardless of its folder or category.

*   `is_read` (boolean): `true` if the user has viewed the message.
*   `is_starred` (boolean): `true` if the user has marked the message as important.

---

## Packet Framing & Transport

* **Framing:** 4-byte big-endian length prefix, then JSON.
* **Transport:** Raw TCP with **mandatory TLS**.
* **Reliability:** Handle partial frames, connection drops, session resumption.

---

## Handshake

**Purpose**: Establish a connection and negotiate protocol version, feature capabilities, and anti-spam requirements before any user-specific operations.

* **identity**: Domain or user entity initiating the connection; For Server -> Server this will be the domain name; For Client -> Server this will be the user's email address
* **encryption**: End-to-end encryption support flag (not available in v1).
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
    "options": {
      "encryption": false
    },
    "supported_anti_spam": ["hashcash"]
  },
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
      "SEND_EMAIL": { "type": "hashcash", "bits": 22 },
      "FETCH_KEYS": { "type": "none" },
      "FETCH_EMAIL":{"type": "hashcash", "bits": 22}
    },
    "options": {
      "encryption": false
    }
  },
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
}
```

### AUTH\_ACK  (Server -> Client)

```json
{
  "type": "AUTH_ACK",
  "timestamp": "2025-06-22T17:10:01Z",
  "payload": {
    "accepted": true,
    "session": { "expires_in": 3600, "identity": "alice~quillmail.com" }
  },
  "signature": "...",
}
```

---

## Sending Emails

**Purpose**: Transfer email messages in a structured manner once the client is authenticated.

### SEND\_EMAIL  (Client/Server -> Server)

**Fields in `SEND_EMAIL.payload`:**

* **message_id**: A unique identifier for the message.
* **thread_id**: Identifier for grouping messages; empty string starts a new thread.
* **from**: The sender's email address.
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
  * `link`: URL or reference for retrieval; all attachments are link-only.
* **options**:
  * `expires_in_seconds`: Time-to-live after which the server may delete or expire the message automatically.
  * `one_time`: Boolean; if `true`, the server **SHOULD** delete the message after a single successful fetch. Note: malicious providers could ignore this and retain messages indefinitely.


```json
{
  "type": "SEND_EMAIL",
  "timestamp": "2025-06-22T17:20:00Z",
  "payload": {
    "message_id": "msg-123",
    "thread_id": "thread-495abc7d",
    "from": "bob~quillmail.xyz",
    "to": ["omer~quillmail.xyz"],
    "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
    "bcc": ["eve~quillmail.xyz"],
    "subject": "Test email example",
    "body": {
      "text": "This is a text message!",
      "html": "<p>Hi <strong>Bob</strong>,<br>See below.</p>"
    },
    "attachments": [
      { "filename": "diagram.png", "mimetype": "image/png", "link": "https://cdn.example.com/diagram.png" }
    ],
    "options": {
      "expires_in_seconds": 7200,
      "one_time": true,
    }
  },
  "signature": "...",
  "anti_spam": { "type": "hashcash", "resource": "msg-123", "bits": 22, "nonce": "000abc123"}
}
```

### SEND\_EMAIL\_ACK  (Server -> Server/Client)

```json
{
  "type": "SEND_EMAIL_ACK",
  "timestamp": "2025-06-22T17:20:03Z",
  "payload": {
    "status": "OK",
    "message_id": "msg-123",
    "delivered_to": ["omer~quillmail.xyz"]
  },
  "signature": "..."
}
```

---

## Fetching Emails

**Purpose**: Retrieve conversation thread overviews and full message threads with pagination support.

### FETCH\_EMAIL Overview (Client -> Server)

**Fields in `FETCH_EMAIL.payload`:**

* **mode**: Must be `"overview"`.
* **folder**: Name of the folder (e.g., `"inbox"`).
* **limit**: Maximum number of threads to return.
* **offset**: Pagination offset (number of threads to skip).
* **filters** (optional): Object to restrict result set:

  * **search**: Full-text search options:

    * **keywords**: List of words to match.
    * **exact\_phrase**: Exact string to match.
    * **from**: List of sender addresses.
    * **to**: List of recipient addresses.
  * **flags**: Message flag filters:

    * **has\_attachments**: `true` or `false`.
    * **is\_read**: `true` or `false`.
    * **is\_starred**: `true` or `false`.
  * **date\_range**: Time range filters:

    * **after**: ISO-8601 timestamp; include messages sent after this time.
    * **before**: ISO-8601 timestamp; include messages sent before this time.

```json
{
  "type": "FETCH_EMAIL",
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
            "from":           ["alice~…"],
            "to":             ["itamar~…"],
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
  "anti_spam": { "type": "hashcash", "resource": "omer~quillmail.xyz", "bits": 22, "nonce": "000abc123"}
}
```

### FETCH\_EMAIL\_RESPONSE Overview (Server -> Client)

**Fields in `FETCH_EMAIL_RESPONSE.payload`:**

* **mode**: Echoes `"overview"`.
* **threads**: Array of thread summaries:

  * **thread\_id**: Conversation identifier.
  * **latest\_message**: Summary of the most recent message:

    * **id**, **thread\_id**, **from**, **subject**, **timestamp**, **snippet**, **flags**.
  * **count**: Total messages in the thread within the folder.
  * **unread\_count**: Number of unread messages in that thread.
* **total\_threads**: Total number of threads available.
* **limit**, **offset**: Echo pagination parameters.

```json
{
  "type": "FETCH_EMAIL_RESPONSE",
  "timestamp": "2025-06-22T20:00:01Z",
  "payload": {
    "status": "OK",
    "mode": "overview",
    "threads": [
      {
        "thread_id": "thread-495abc7d",
        "latest_message": {
          "id": "msg-8241e",
          "thread_id": "thread-495abc7d",
          "from": "bob~quillmail.com",
          "subject": "Re: Lunch tomorrow?",
          "timestamp": "2025-06-22T18:50:00Z",
          "snippet": "Sure, let's meet at noon.",
          "flags": {
            "has_attachments": true,
            "is_starred": true,
          },
        },
        "count": 5,
        "unread_count": 2
      }
    ],
    "total_threads": 42,
    "limit": 20,
    "offset": 0
  },
  "signature": "...",
}
```

---

### FETCH\_EMAIL Thread  (CLient -> Server)

**Fields in `FETCH_EMAIL.payload`:**

* **mode**: Must be `"thread"`.
* **thread\_id**: Identifier of the conversation thread.
* **limit**: Maximum number of messages to return.
* **offset**: Pagination offset (number of messages to skip).

```json
{
  "type": "FETCH_EMAIL",
  "timestamp": "2025-06-22T20:05:00Z",
  "payload": {
    "mode": "thread",
    "thread_id": "thread-495abc7d",
    "limit": 5,
    "offset": 0
  },
  "signature": "...",
  "anti_spam": { "type": "hashcash", "resource": "alice~quillmail.com", "bits": 22, "nonce": "000abc123"},
}
```

### FETCH\_EMAIL\_RESPONSE Thread (Server -> Client)

**Fields in `FETCH_EMAIL_RESPONSE.payload`:**

* **status**: `"OK"` or `"ERROR"`.
* **mode**: Echoes `"thread"`.
* **thread\_id**: Conversation identifier.
* **messages**: Array of full message objects (`MessageDTO`):

  * **id**, **from**, **to**, **cc**, **bcc**, **subject**
  * **body**: `{ "text": ..., "html": ... }`
  * **attachments**: Array of `{ filename, mimetype, link }`.
  * **timestamp**, **flags**.
* **total\_messages**: Total messages in the thread.
* **limit**, **offset**: Echo pagination parameters.

```json
{
  "type": "FETCH_EMAIL_RESPONSE",
  "timestamp": "2025-06-22T20:05:01Z",
  "payload": {
    "status": "OK",
    "mode": "thread",
    "thread_id": "thread-495abc7d",
    "messages": [
      {
        "id": "msg-8241d",
        "from": "alice~quillmail.com",
        "to": ["bob~quillmail.com"],
        "cc": [],
        "bcc": [],
        "subject": "Lunch tomorrow?",
        "body": {
          "text": "Hey Bob, are you free for lunch tomorrow?",
          "html": "<p>Hey <strong>Bob</strong>, are you free for lunch tomorrow?</p>"
        },
        "attachments": [
          { "filename": "menu.pdf", "mimetype": "application/pdf", "link": "https://cdn.quillmail.com/menu.pdf" }
        ],
        "timestamp": "2025-06-22T18:45:00Z",
        "flags": {
            "is_read": false,
            "is_starred": true,
        },
      }
    ],
    "total_messages": 5,
    "limit": 5,
    "offset": 0
  },
    "signature": "...",
}
```

---

## Key Management and Federation

This section describes how user keys are managed by providers and how public keys are discovered across the federated network.

### Key Management Strategies

The Quill protocol supports two primary models for handling user cryptographic keys. The choice of which model to implement is left to the individual service provider.

*   **Server-Managed Keys**: The provider generates and stores private keys for its users (e.g., encrypted in a database). In this model, the server handles all signing operations on the user's behalf after they authenticate. This is simpler for the end-user.
*   **Bring-Your-Own-Key (BYOK)**: The user generates their own key pair locally and uploads only the public key to the server. The user's client application signs all outgoing packets, and the server's role is simply to verify the signature before relaying the message. This offers greater security and control to the user.

These key management strategies are implementation-specific choices for the provider and are not directly enforced by the protocol itself. The protocol only cares that a valid signature is present on authenticated packets.

### Public Key Discovery for Federation

To enable a decentralized network where anyone can verify anyone else's messages, servers need a way to look up the public keys of users on other domains. The `FETCH_KEYS` command facilitates this process.

`FETCH_KEYS` is designed exclusively for **server-to-server** communication. Client applications **SHOULD NOT** use this command directly.

#### Server-to-Server Trust Model

The trust for `FETCH_KEYS` is established at the transport layer. A server receiving a `FETCH_KEYS` request authenticates the requesting server using its **CA-signed TLS certificate**.

The receiving server **MUST** perform the following checks:

1.  **Certificate Validity:** The certificate must be valid and signed by a trusted Certificate Authority (CA). Self-signed certificates **MUST** be rejected.
2.  **Identity Match:** The domain identity claimed in the `HANDSHAKE` packet (e.g., `identity: "provider-b.com"`) **MUST** match a Subject Alternative Name (SAN) or the Common Name (CN) in the peer's TLS certificate.

#### Server Responsibility & Abuse Prevention

Because this endpoint reveals the existence of user accounts, each server operator is responsible for protecting its users from enumeration attacks and for acting as a good citizen on the network. This responsibility includes:

*   **Behavioral Monitoring:** Servers **MUST** monitor the rate of `FETCH_KEYS` requests from peer servers. An abnormally high volume of requests indicates a potential attack or compromised server, which should be temporarily or permanently blocked.
*   **Federation Trust Policy:** Servers **SHOULD** maintain a trust policy (e.g., a blacklist of peer certificates or domains) to immediately cut off communication with known bad actors.


---

### FETCH\_KEYS  (Server -> Server)

```json
{
  "type": "FETCH_KEYS",
  "timestamp": "2025-06-22T17:30:00Z",
  "payload": { "query": "alice~quillmail.com" },
}
```

### KEY\_RESPONSE  (Server -> Server)

```json
{
  "type": "KEY_RESPONSE",
  "timestamp": "2025-06-22T17:30:01Z",
  "payload": {
    "email": "alice~quillmail.com",
    "public_key": "base64-public-key",
    "expires": "2026-01-01T00:00:00Z"
  },
}
```

## Error Handling

All errors use a unified `ERROR` packet<br>
The signature field is only present if the error occurs after a client has successfully authenticated. Pre-authentication errors (like a failed handshake) are not signed.

```json
{
  "type": "ERROR",
  "timestamp": "2025-06-22T17:40:00Z",
  "payload": {
    "code": "RATE_LIMITED",
    "message": "Too many requests.",
    "context": "SEND_EMAIL",
    "temporary": true,
    "retry_after": 60
  },
  "signature": "...",
}
```

---

### **Error Code Categories**

This section details the standardized error codes for the Quill protocol. All errors are sent within a unified `ERROR` packet.

#### **1. General & Protocol Errors**
*These errors can occur at any stage of the connection.*

*   `MALFORMED_PACKET`: The received packet could not be parsed. This could be due to an invalid length prefix, non-compliant JSON, or missing required top-level fields (`type`, `timestamp`, `payload`).
*   `INVALID_TIMESTAMP`: The `timestamp` field is outside the acceptable ±60-second skew, or its format is invalid.
*   `RATE_LIMITED`: The client or server has exceeded the allowed number of requests in a given time frame. The `retry_after` field SHOULD be included.
*   `TOO_MANY_CONNECTIONS`: The server is unable to accept new connections from the client's IP address or identity.
*   `REQUEST_TIMEOUT`: The server timed out waiting for a packet from the client.
*   `INTERNAL_SERVER_ERROR`: A generic error for an unexpected condition on the server. The `temporary` flag should indicate if retrying is likely to succeed.
*   `PROTOCOL_DISABLED`: The Quill protocol has been disabled on this server.

#### **2. Handshake Errors**
*Errors that occur during the initial `HANDSHAKE` and `HANDSHAKE_ACK` exchange.*

*   `UNSUPPORTED_VERSION`: The protocol `version` requested by the initiator is not supported by the server.
*   `TLS_REQUIRED`: The connection is not secured with TLS, which is mandatory.
*   `IDENTITY_MISMATCH`: (Server-to-Server) The `identity` in the `HANDSHAKE` payload does not match the Common Name (CN) or a Subject Alternative Name (SAN) in the peer's TLS certificate.
*   `SPAM_POLICY_MISMATCH`: The anti-spam methods supported by the initiator are incompatible with the server's requirements.

#### **3. Authentication & Authorization Errors**
*Errors related to client identity, credentials, and permissions.*

*   `AUTH_REQUIRED`: The client attempted an operation that requires authentication without having an active, authenticated session.
*   `UNSUPPORTED_METHOD`: The `AUTH` `method` (e.g., `session_token`) is not supported by the server.
*   `INVALID_TOKEN` or `CREDENTIALS_REJECTED`: The provided credentials (e.g., token, password hash) are invalid or have expired.
*   `TOO_MANY_ATTEMPTS`: The client has made too many failed authentication attempts.
*   `CHALLENGE_FAILED`: The client failed a server-issued authentication challenge (if using a challenge-response mechanism).
*   `SENDER_MISMATCH`: An authenticated client tried to send an email (`SEND_EMAIL`) where the `from` address does not belong to the authenticated identity.
*   `PERMISSION_DENIED`: The authenticated user or peer server is not authorized to perform the requested action (e.g., a client trying to use `FETCH_KEYS`).

#### **4. Message & Data Transfer Errors (SEND/FETCH)**
*Errors related to creating, sending, or retrieving emails.*

*   `INVALID_SIGNATURE`: The signature on a post-authentication packet is missing, invalid, or does not match the user's public key.
*   `SIGNATURE_REQUIRED`: A packet that requires a signature was sent without one.
*   `INVALID_RECIPIENTS`: One or more addresses in the `to`, `cc`, or `bcc` fields are malformed or invalid.
*   `RECIPIENT_UNAVAILABLE`: One or more recipient addresses do not exist on the destination server. (This would typically be sent from a receiving server back to a sending server/client).
*   `TOO_LARGE`: The message payload exceeds the server's size limits.
*   `BLOCKED_DOMAIN`: The message contains a recipient on a domain that is blocked by the server's policy.
*   `FOLDER_NOT_FOUND`: The `folder` specified in a `FETCH_EMAIL` request does not exist.
*   `THREAD_NOT_FOUND`: The `thread_id` specified in a `FETCH_EMAIL` request does not exist.
*   `INVALID_FILTER`: The `filters` object in a `FETCH_EMAIL` request is malformed or contains unsupported criteria.
*   `INVALID_MODE`: The `mode` in a `FETCH_EMAIL` request is not one of the supported values (e.g., `"overview"`, `"thread"`).

#### **5. Anti-Spam Errors**
*Errors specific to the anti-spam negotiation and proof validation.*

*   `SPAM_PROOF_REQUIRED`: The packet is missing the `anti_spam` object required for this operation (as negotiated in the `HANDSHAKE_ACK`).
*   `INVALID_SPAM_PROOF`: The provided anti-spam proof is invalid. For `hashcash`, this could mean the hash doesn't have the required number of leading zeros, or the `resource` field does not match the required value (e.g., `message_id` for `SEND_EMAIL`).
*   `SPAM_DETECTED`: The message was rejected by server-side content analysis, independent of the negotiated anti-spam proof.

#### **6. Federation & Key Management Errors**
*Errors specific to server-to-server interactions, primarily for `FETCH_KEYS`.*

*   `USER_NOT_FOUND`: The user identity in a `FETCH_KEYS` `query` does not exist on the server.
*   `FEDERATION_DENIED`: The peer server is blocked based on the local server's trust policy (e.g., its domain or certificate is on a blocklist).

---