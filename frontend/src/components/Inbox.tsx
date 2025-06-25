// frontend/src/components/Unread.tsx

import React, { useState, useEffect } from 'react';
import { MessageOverview, FetchResponsePacket , FetchResponseSuccessPayload,FetchResponseErrorPayload} from '../types/quill'; // Types for your protocol
import { User } from 'firebase/auth'; // Type for Firebase user
import '../css/MailBox.css'; // Your CSS for styling this view
import Content from './Content'; // The component that will display the email's content

interface InboxProps {
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
      },
      {
        "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
        "thread_id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
        "from": "newsletter~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": [],
        "subject": "Weekly Digest - Read",
        "body": {
          "content": [
            { "type": "text/plain", "value": "Here is your weekly digest of news and articles." }
          ]
        },
        "timestamp": "2025-06-20T12:00:00.000+03:00",
        "read": true
      },
      {
        "id": "f0e9d8c7-b6a5-4321-fedc-ba9876543210",
        "thread_id": "f0e9d8c7-b6a5-4321-fedc-ba9876543210",
        "from": "support~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": [],
        "subject": "Your support ticket has been updated - Unread 4",
        "body": {
          "content": [
            { "type": "text/plain", "value": "We have an update regarding your recent support ticket #12345." }
          ]
        },
        "timestamp": "2025-06-22T14:00:00.000+03:00",
        "read": false
      },
      {
        "id": "12345678-1234-1234-1234-1234567890ab",
        "thread_id": "12345678-1234-1234-1234-1234567890ab",
        "from": "marketing~quillmail.xyz",
        "to": ["omer~quillmail.xyz"],
        "cc": [],
        "subject": "Special Offer Just For You! - Unread 5",
        "body": {
          "content": [
            { "type": "text/html", "value": "<h1>Don't miss out!</h1><p>Check out our latest deals.</p>" }
          ]
        },
        "timestamp": "2025-06-23T10:30:00.000+03:00",
        "read": false
      },
      {
         "id": "a1b2c3d4-e5f6-7890-1234-567890abcde1",
         "thread_id": "a1b2c3d4-e5f6-7890-1234-567890abcde1",
         "from": "billing~quillmail.xyz",
         "to": ["omer~quillmail.xyz"],
         "cc": [],
         "subject": "Your Invoice #54321 - Unread 9",
         "body": {
           "content": [
             { "type": "text/plain", "value": "Your recent invoice is attached. Thank you for your business." }
           ]
         },
         "timestamp": "2025-06-25T08:00:00.000+03:00",
         "read": false
       },
       {
         "id": "a1b2c3d4-e5f6-7890-1234-567890abcde2",
         "thread_id": "a1b2c3d4-e5f6-7890-1234-567890abcde2",
         "from": "travel-agency~quillmail.xyz",
         "to": ["omer~quillmail.xyz"],
         "cc": [],
         "subject": "Your Flight Itinerary - Read",
         "body": {
           "content": [
             { "type": "text/html", "value": "<p>Your flight details for your upcoming trip are confirmed. Have a safe journey!</p>" }
           ]
         },
         "timestamp": "2025-06-17T11:30:00.000+03:00",
         "read": true
       },
       {
         "id": "a1b2c3d4-e5f6-7890-1234-567890abcde3",
         "thread_id": "a1b2c3d4-e5f6-7890-1234-567890abcde3",
         "from": "conference-updates~quillmail.xyz",
         "to": ["omer~quillmail.xyz"],
         "cc": [],
         "subject": "Speaker Announcement for TechCon 2025 - Unread 10",
         "body": {
           "content": [
             { "type": "text/plain", "value": "We are thrilled to announce our keynote speaker for TechCon 2025! More details inside." }
           ]
         },
         "timestamp": "2025-06-25T10:15:00.000+03:00",
         "read": false
       },
       {
         "id": "a1b2c3d4-e5f6-7890-1234-567890abcde4",
         "thread_id": "a1b2c3d4-e5f6-7890-1234-567890abcde4",
         "from": "online-retailer~quillmail.xyz",
         "to": ["omer~quillmail.xyz"],
         "cc": [],
         "subject": "Your Order has Shipped! - Unread 11",
         "body": {
           "content": [
             { "type": "text/plain", 
              "value": "Great news! Your order #98765 has been shipped and is on its way to you."
              }
           ]
         },
         "timestamp": "2025-06-24T18:00:00.000+03:00",
         "read": false
       },
       {
         "id": "a1b2c3d4-e5f6-7890-1234-567890abcde5",
         "thread_id": "a1b2c3d4-e5f6-7890-1234-567890abcde5",
         "from": "social-network~quillmail.xyz",
         "to": ["omer~quillmail.xyz"],
         "cc": [],
         "subject": "You have a new connection request - Unread 12",
         "body": {
           "content": [
             { "type": "text/html",
               "value": "<p>Someone wants to connect with you on QuillNet. Click here to view their profile.</p> 3helllllllllllllllll                                                                                                                      kkkkkkkkkkkkkkkkkkkkkkkkk ooooooooooooooooooooooooooooooooooooooooooooooooo     kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk       kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk kkkkkkkkkkkkkkkkkkk oooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk" }
           ]
         },
         "timestamp": "2025-06-25T11:45:00.000+03:00",
         "read": false
       }
     ],
     "total": 12,
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


const Inbox: React.FC<InboxProps> = ({ user }) => {
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
        setMessages(successPayload.messages);
        
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
    return <div>Loading messages...</div>;
  }

  if (error) {
    return <div style={{ color: 'red' }}>Error: {error}</div>;
  }

  const messageListContent = messages.length === 0 ? (<div className="empty-message-list-panel">No messages (from mock).</div>) :
    (
    <div className="message-list">
        {messages.map(message => (
          <button
            key={message.id}
            className={`message-item ${selectedMessage?.id === message.id ? 'selected' : ''}`}
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
    <div className="view-layout">
        <div className="message-list-panel">
            {messageListContent}
        </div>
        <div className="mail-content-panel">
            <Content selectedMessage={selectedMessage} />
        </div>
    </div>
  );
};

export default Inbox;