import "../css/Content.css"; 

import { User } from 'firebase/auth'; // Importing User type from Firebase Auth


interface ContentProps {
  user: User | null; // 'user' can be a Firebase User object OR null
  handleSignOut: () => Promise<void>; // 'handleSignOut' is a function returning a Promise that resolves to void
}

function Content({ user, handleSignOut }: ContentProps) {
  return (
      <h1>hello</h1>
  );
}

export default Content;