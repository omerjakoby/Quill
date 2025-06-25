// frontend/src/components/Content.tsx

import React from 'react';
import { MessageOverview } from '../types/quill'; // Import the type for a message overview

// 1. Define the props interface for the Content component
interface ContentProps {
  selectedMessage: MessageOverview | null; // Content expects a selectedMessage prop, which can be a MessageOverview object or null.
}

// 2. Type the Content component using React.FC and ContentProps
const Content: React.FC<ContentProps> = ({ selectedMessage }) => {
  // If no message is selected (initial state or empty list), display a placeholder message.
  if (!selectedMessage) {
    return (
      <div className="mail-content-empty"> {/* Class for styling the empty state */}
        <p>No email selected. Please choose an email from the list.</p>
      </div>
    );
  }

  // Safely find the HTML content (if available) from the message body
  const htmlContent = selectedMessage.body.content.find(
    (item) => item.type === 'text/html'
  )?.value;

  // Safely find the plain text content (if available) from the message body
  const plainTextContent = selectedMessage.body.content.find(
    (item) => item.type === 'text/plain'
  )?.value;

  // Render the detailed view of the selected message
  return (
    <div className="mail-content-display"> {/* Main container for the detailed mail view */}
      <div className="mail-header"> {/* Header section of the email view */}
        <h2>{selectedMessage.subject}</h2> {/* Subject of the email */}
        <p>From: <strong>{selectedMessage.from}</strong></p> {/* Sender's address */}
        <p>To: {selectedMessage.to.join(', ')}</p> {/* Recipients, joined by comma */}
        {/* Conditionally render CC recipients if they exist and the array is not empty */}
        {selectedMessage.cc && selectedMessage.cc.length > 0 && (
          <p>Cc: {selectedMessage.cc.join(', ')}</p>
        )}
        <span className="mail-timestamp">
          {new Date(selectedMessage.timestamp).toLocaleString()} {/* Formatted timestamp */}
        </span>
      </div>

      <div className="mail-body"> {/* Body section of the email view */}
        {/* Conditionally display HTML content if it's available, otherwise fallback to plain text or a default message */}
        {htmlContent ? (
          // `dangerouslySetInnerHTML`: React's way to render raw HTML. It's "dangerous" because if the HTML
          // comes from untrusted sources, it can expose your app to Cross-Site Scripting (XSS) attacks.
          // Use with caution and only for trusted content.
          <div dangerouslySetInnerHTML={{ __html: htmlContent }} />
        ) : (
          // Fallback to plain text if HTML content is not found, or show a generic message
          <p>{plainTextContent ?? 'No message body available.'}</p>
        )}
      </div>
      {/* Optional: Add buttons for Reply, Forward, Delete, etc. */}
    </div>
  );
};

export default Content;