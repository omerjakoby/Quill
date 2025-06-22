// frontend/src/types/quill.ts

/**
 * Base interface for all Quill protocol packets.
 * All packets must adhere to this common structure.
 */
interface QuillBasePacket {
  protocol: "quill";
  version: "1.0";
  timestamp: string; // ISO 8601 format, e.g., "2025-06-17T15:45:12Z"
}

// --- PING Protocol ---

/**
 * Request packet for a PING operation.
 * Used to check server liveness and session validity.
 */
export interface PingPacket extends QuillBasePacket {
  type: "PING";
  session_token: string;
  payload: {}; // Empty payload for PING
}

/**
 * Response packet for a PING operation.
 * Provides server status and session validity.
 */
export interface PingResponsePacket extends QuillBasePacket {
  type: "PING_RESPONSE";
  payload: {
    status: "OK";
    server_time: string; // Current server time in ISO 8601 format
    session_valid: boolean; // Indicates if the provided session_token is still valid
  };
}

// --- FETCH Protocol ---

/**
 * Payload for fetching message overviews (e.g., inbox list).
 */
export interface FetchOverviewPayload {
  mode: "overview";
  folder: "INBOX" | "SENT" | "DRAFTS" | "TRASH" | "ARCHIVED"; // Specify allowed folders
  limit: number; // Max number of messages to return
  offset: number; // Starting offset for pagination
}

/**
 * Payload for fetching a specific message thread.
 */
export interface FetchThreadPayload {
  mode: "thread";
  thread_id: string; // ID of the thread to fetch
}

/**
 * Request packet for a FETCH operation.
 * Can fetch either overview (list) or a specific thread.
 */
export interface FetchPacket extends QuillBasePacket {
  type: "FETCH";
  // The payload can be either FetchOverviewPayload or FetchThreadPayload
  payload: FetchOverviewPayload | FetchThreadPayload;
  session_token?: string; // Session token might be optional depending on your auth flow
}

/**
 * Structure for a single message overview (returned in FETCH_RESPONSE overview mode).
 */
export interface MessageOverview {
  id: string;
  thread_id: string;
  from: string;
  subject: string;
  snippet: string; // A short summary of the message body
  timestamp: string; // ISO 8601 date string
  is_read: boolean;
  has_attachment: boolean;
}

/**
 * Payload for a successful FETCH_RESPONSE.
 */
export interface FetchResponseSuccessPayload {
  status: "OK";
  mode: "overview"; // Or 'thread' if thread mode was requested
  messages: MessageOverview[]; // List of messages for overview mode
  total: number; // Total number of messages in the folder/thread
  limit: number;
  offset: number;
  // TODO: Add full message/thread payload if mode is 'thread'
}

/**
 * Payload for an erroneous FETCH_RESPONSE.
 */
export interface FetchResponseErrorPayload {
  status: "ERROR";
  code: string; // e.g., "INVALID_FOLDER", "ACCESS_DENIED"
  message: string;
}

/**
 * Response packet for a FETCH operation.
 */
export interface FetchResponsePacket extends QuillBasePacket {
  type: "FETCH_RESPONSE";
  // The payload can be either a success or an error structure
  payload: FetchResponseSuccessPayload | FetchResponseErrorPayload;
}


// --- SEND Protocol ---

/**
 * Represents a single piece of content within an email body.
 */
export interface EmailBodyContent {
  type: "text/plain" | "text/html"; // Allowed content types
  value: string; // The actual content (plain text or HTML string)
}

/**
 * Represents an attachment in an email.
 */
export interface EmailAttachment {
  filename: string;
  mimetype: string; // e.g., "image/jpeg", "application/pdf"
  content_base64: string; // Base64 encoded content of the attachment
}

/**
 * Optional options for sending an email.
 */
export interface SendOptions {
  expires_in_seconds?: number; // Message expiry time
  one_time?: boolean; // Message can be read only once
  thread_id?: string; // Associate with an existing thread
}

/**
 * Request packet for a SEND operation (sending an email).
 */
export interface SendPacket extends QuillBasePacket {
  type: "SEND";
  session_token: string;
  payload: {
    to: string[];
    cc?: string[];
    bcc?: string[];
    subject: string;
    body: {
      content: EmailBodyContent[];
    };
    attachments?: EmailAttachment[];
    options?: SendOptions;
  };
}

/**
 * Payload for a successful SEND_RESPONSE.
 */
export interface SendResponseSuccessPayload {
  status: "OK";
  message_id: string; // Unique ID of the sent message
  thread_id: string; // ID of the thread the message belongs to
  delivered_to: string[]; // List of recipients successfully delivered to
  queued_for: string[]; // List of recipients queued for future delivery attempts
}

/**
 * Payload for an erroneous SEND_RESPONSE.
 */
export interface SendResponseErrorPayload {
  status: "ERROR";
  code: string; // e.g., "INVALID_RECIPIENTS", "MESSAGE_TOO_LARGE"
  message: string;
}

/**
 * Response packet for a SEND operation.
 */
export interface SendResponsePacket extends QuillBasePacket {
  type: "SEND_RESPONSE";
  payload: SendResponseSuccessPayload | SendResponseErrorPayload;
}