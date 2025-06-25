// frontend/src/components/Unread.tsx

import React, { useState, useEffect } from 'react';
import { MessageOverview, FetchResponsePacket , FetchResponseSuccessPayload,FetchResponseErrorPayload} from '../types/quill'; // Types for your protocol
import { User } from 'firebase/auth'; // Type for Firebase user
import '../css/MailBox.css'; // Your CSS for styling this view
import '../css/Unread.css'; // Specific styles for the Unread component
import Content from './Content'; // The component that will display the email's content

interface UnreadProps {
  user: User | null; // Passed from parent (MainWebsite.tsx) if needed
}

// --- MOCK DATA FOR YOUR INBOX ---
// This large string holds the JSON data that simulates a response from your backend.
// It includes several messages, some read, some unread, to demonstrate filtering and display.
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
        "subject": "Test email example - Unread 1",
        "body": {
          "content": [
            { "type": "text/plain", "value": "This is the plain text body for Unread 1." },
            { "type": "text/html", "value": "<p>This is the HTML body for <strong>Unread 1</strong> with some <span style='color: blue;'>blue text</span>!</p>" }
          ]
        },
        "timestamp": "2025-06-22T09:13:49.976+03:00",
        "read": false
      },
      {
        "id": "406e7548-14c0-4b2f-b530-3abf7131a492",
        "thread_id": "ab391508-2826-4f9f-8301-397f7396b09b",
        "from": "alice~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": [],
        "subject": "Important Meeting - Unread 2",
        "body": {
          "content": [
            { "type": "text/plain", 
             "value": "Hi Omer, please find the meeting details attached for tomorrow." }
          ]
        },
        "timestamp": "2025-06-22T09:12:37.372+03:00",
        "read": false
      },
      {
        "id": "e9ca7933-5b3d-4e5f-91ba-0aaeb1f42648",
        "thread_id": "8576661f-5dc4-43ad-8eb0-b69d303b48b7",
        "from": "charlie~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": ["diana~quillmail.xyz"],
        "subject": "Your Subscription - Read (Should Not Show)",
        "body": {
          "content": [
            { "type": "text/plain", "value": "Your monthly subscription has been renewed. Thank you for your payment." }
          ]
        },
        "timestamp": "2025-06-22T09:00:30.651+03:00",
        "read": true 
      },
      {
        "id": "b55e3c4b-66b8-452a-9118-d9fe2fa52141",
        "thread_id": "34159dae-69d4-4113-b7e4-8f15e660a155",
        "from": "info~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": [],
        "subject": "New Features Rolled Out! - Unread 3",
        "body": {
          "content": [
            { "type": "text/plain", "value": "We're excited to announce new features..." }
          ]
        },
        "timestamp": "2025-06-21T18:27:20.073+03:00",
        "read": false
      }
    ],
    "total": 4,
    "limit": 20
  }
}
`;

// Mock data for an empty inbox scenario
const mockFetchResponseJsonEmpty = `
{
  "protocol": "quill",
  "version": "1.0",
  "type": "FETCH_RESPONSE",
  "timestamp": "2025-06-24T10:30:00Z",
  "payload": {
    "status": "OK",
    "mode": "folder",
    "messages": [],
    "total": 0,
    "limit": 20,
    "offset": 0
  }
}`;


const Unread: React.FC<UnreadProps> = ({ user }) => {
  const [messages, setMessages] = useState<MessageOverview[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedMessage, setSelectedMessage] = useState<MessageOverview | null>(null);

  const handleMessageClick = (message: MessageOverview) => {
    setSelectedMessage(message);
  };

  useEffect(() => {
    try {
      const parsedData: FetchResponsePacket = JSON.parse(mockFetchResponseJson);

      if (parsedData.payload.status === "OK" && parsedData.payload.mode === "folder") {
        const successPayload = parsedData.payload as FetchResponseSuccessPayload;
        const unreadMessages = successPayload.messages.filter(message => !message.read);
        setMessages(unreadMessages);

        if (unreadMessages.length > 0) {
            setSelectedMessage(unreadMessages[0]);
        }
      } else if (parsedData.payload.status === "ERROR") {
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
      setLoading(false);
    }
  }, []);

  if (loading) {
    return <div>Loading unread messages...</div>;
  }

  if (error) {
    return <div style={{ color: 'red' }}>Error: {error}</div>;
  }

  const messageListContent = messages.length === 0 ? (<div className="empty-message-list-panel">No unread messages (from mock).</div>) :
    (
    <div className="message-list">
        {messages.map(message => (
          <button
            key={message.id}
            className={`message-item-unread ${selectedMessage?.id === message.id ? 'selected' : ''}`}
            onClick={() => handleMessageClick(message)}
            type="button"
            tabIndex={0} // Added tabIndex for accessibility
            onKeyDown={(event) => { // Added keyboard handler for accessibility
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                handleMessageClick(message);
              }
            }}
          >
            {!message.read && <span className="unread-dot"></span>}
            <h3 className="message-subject">{message.subject}</h3>
            <p className="message-from">From: {message.from}</p>
            <p className="message-snippet">{message.body.content[0]?.value || 'No snippet available'}</p>
            <span className="message-timestamp">
              {new Date(message.timestamp).toLocaleString()}
            </span>
          </button>
        ))}
    </div>
  );

  return (
    <div className="unread-view-layout">
        <div className="message-list-panel">
            {messageListContent}
        </div>
        <div className="mail-content-panel">
            <Content selectedMessage={selectedMessage} />
        </div>
    </div>
  );
};

export default Unread;