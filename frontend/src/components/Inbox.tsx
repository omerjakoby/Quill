// frontend/src/components/Inbox.tsx
import React, { useState, useEffect } from 'react';
import { MessageOverview, FetchResponsePacket , FetchResponseSuccessPayload,FetchResponseErrorPayload} from '../types/quill'; // Import your Quill Protocol types
import { User } from 'firebase/auth'; // Still need User for InboxProps
import '../css/MailBox.css'; // Import your CSS for styling the inbox

// Define payload types if not already imported


interface InboxProps {
  // Inbox might need user data even for mock for consistency, but not used here directly
  user: User | null;
}

const mockFetchResponseJson = `
{
  "protocol": "quill",
  "version": "1.0",
  "type": "FETCH_RESPONSE",
  "timestamp": "2025-06-22T06:29:26.7114074Z",
  "payload": {
    "status": "OK",
    "mode": "folder",
    "messages": [
      {
        "id": "e7ef3916-efb3-40e3-ae3d-f8c22a934a6a",
        "thread_id": "33ca0d3f-098b-4f13-9ada-271af6dc350f",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-22T09:13:49.976+03:00",
        "read": false
      },
      {
        "id": "406e7548-14c0-4b2f-b530-3abf7131a492",
        "thread_id": "ab391508-2826-4f9f-8301-397f7396b09b",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-22T09:12:37.372+03:00",
        "read": false
      },
      {
        "id": "e9ca7933-5b3d-4e5f-91ba-0aaeb1f42648",
        "thread_id": "8576661f-5dc4-43ad-8eb0-b69d303b48b7",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-22T09:00:30.651+03:00",
        "read": false
      },
      {
        "id": "e147dbc6-d2e7-441b-9636-e776785b79fb",
        "thread_id": "73f3f063-2b8c-45f9-a457-dda23fb66282",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-22T08:59:50.815+03:00",
        "read": true
      },
      {
        "id": "b55e3c4b-66b8-452a-9118-d9fe2fa52141",
        "thread_id": "34159dae-69d4-4113-b7e4-8f15e660a155",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-21T18:27:20.073+03:00",
        "read": false
      },
      {
        "id": "60506e66-f9e5-4f78-bce8-619989c65f2e",
        "thread_id": "9aa262c7-8713-4949-becb-eef18ced63ea",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-21T18:27:17.001+03:00",
        "read": false
      },
      {
        "id": "fa03a122-9c67-44ed-91be-c8c5d52362e6",
        "thread_id": "8f5cf1d6-3fc7-4a66-a0e2-112da1a5125c",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-21T18:25:16.156+03:00",
        "read": false
      },
      {
        "id": "fa03a122-9c67-44ed-91be-c8c5d52362e7",
        "thread_id": "8f5cf1d6-3fc7-4a66-a0e2-112da1a5125c",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-21T18:25:16.156+03:00",
        "read": true
      },
      {
        "id": "fa03a122-9c67-44ed-91be-c8c5d52362e9",
        "thread_id": "8f5cf1d6-3fc7-4a66-a0e2-112da1a5125c",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-21T18:25:16.156+03:00",
        "read": true
      },
      {
        "id": "fa03a122-9c67-44ed-91be-c8c5d52362e4",
        "thread_id": "8f5cf1d6-3fc7-4a66-a0e2-112da1a5125c",
        "from": "bob~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["carol~quillmail.xyz", "Dave~quillmail.xyz"],
        "subject": "Test email example",
        "body": {
          "content": [
            { "type": "", "value": "This is a text message!" },
            { "type": "", "value": "\\u003cp\\u003eHi \\u003cstrong\\u003eBob\\u003c/strong\\u003e,\\u003cbr\\u003eSee below.\\u003c/p\\u003e" }
          ]
        },
        "timestamp": "2025-06-21T18:25:16.156+03:00",
        "read": true
      }
    ],
    "total": 10,
    "limit": 20
  }
}`;

const mockFetchResponseJsonEmpty=`
{
  "protocol": "quill",
  "version": "1.0",
  "type": "FETCH_RESPONSE",
  "timestamp": "2025-06-24T10:30:00Z",
  "payload": {
    "status": "OK",
    "mode": "folder",
    "messages": [],  // <-- THIS IS THE KEY PART: an empty array
    "total": 0,      // <-- And ideally, total should also be 0
    "limit": 20,
    "offset": 0
  }
}`;


const Inbox: React.FC<InboxProps> = ({ user }) => {
  const [messages, setMessages] = useState<MessageOverview[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // We don't need the user check for mock data
    // if (!user) {
    //   setLoading(false);
    //   setError("User not authenticated for inbox.");
    //   return;
    // }
    // Inside useEffect in Inbox.tsx
    
    try {
      const parsedData: FetchResponsePacket = JSON.parse(mockFetchResponseJson);

      if (parsedData.payload.status === "OK" && parsedData.payload.mode === "folder") {
        // Cast to FetchResponseSuccessPayload to access 'messages'
        const successPayload = parsedData.payload as FetchResponseSuccessPayload;
        setMessages(successPayload.messages);
      } else if (parsedData.payload.status === "ERROR") {
        // Cast to FetchResponseErrorPayload to access 'message'
        const errorPayload = parsedData.payload as FetchResponseErrorPayload;
        setError(`Error fetching messages: ${errorPayload.message}`);
      } else {
        setError("Unexpected response format from mock data.");
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(`Failed to parse mock data: ${err.message}`);
      } else {
        setError("An unknown error occurred parsing mock data.");
      }
    } finally {
      setLoading(false); // Ensure loading state is false after processing
    }
  }, []); // Empty dependency array, runs once on mount for mock data

  if (loading) {
    return <div>Loading inbox...</div>;
  }

  if (error) {
    return <div style={{ color: 'red' }}>Error: {error}</div>;
  }

  if (messages.length === 0) {
    return <div>Your inbox is empty (from mock).</div>;
  }

  return (
    <div className="inbox-view">
      <div className="message-list-inbox">
        {messages.map(message => (
          <div key={message.id} className="message-item">
            <h3 className="message-subject">{message.subject}</h3>
            <p className="message-from">From: {message.from}</p>
            <span className="message-timestamp">
              {new Date(message.timestamp).toLocaleString()}
            </span>
            <span className="message-read-status">
              {message.read ? '  Read' : '  Unread'}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default Inbox;