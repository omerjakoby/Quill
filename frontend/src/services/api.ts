import {
  QuillBasePacket,
  PingPacket,
  PingResponsePacket,
  FetchPacket,
  FetchOverviewPayload,
  FetchResponsePacket,
  SendPacket,
  SendResponsePacket,
  // ... and any other packet types you define later, e.g., FetchThreadPacket
} from '../types/quill';

const API_BASE_URL = 'http://localhost:9876/api/quill'

/**
 * A generic function to send any Quill Protocol request packet
 * and receive its corresponding response packet.
 *
 * @param packet - The Quill Protocol request packet to send.
 * @returns A Promise that resolves with the response packet.
 * @throws An Error if the network request fails or the server returns an HTTP error.
 */


