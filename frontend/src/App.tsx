// src/App.jsx

import { useState, useEffect } from "react";
import { auth } from "./services/firebaseConfig.ts";
import { onAuthStateChanged, signOut, User } from "firebase/auth";

// Import our two main pages
import LoginPage from "./pages/login";
import MainWebsite from "./pages/mainWebsite";

function App() {
  const [user, setUser] = useState<User|null>(null); // Holds user data if logged in
  const [loading, setLoading] = useState<boolean>(true); // Shows a loading state initially

  // This is the core of our logic. It listens for changes in login state.
  useEffect(() => {
    const unsubscribe = onAuthStateChanged(auth, (currentUser) => {
      setUser(currentUser);
      setLoading(false);
    });

    // This cleans up the listener when the app closes
    return () => unsubscribe();
  }, []); // The empty array means this effect runs only once

  // A simple function to sign the user out
  const handleSignOut = async () => {
    try {
      await signOut(auth);
    } catch (error) {
      console.error("Sign out error:", error);
    }
  };

  // While Firebase is checking the user's status, show a loading message.
  if (loading) {
    return <div>Loading...</div>;
  }

  // This is the main decision:
  // If 'user' has data, show the MainWebsite.
  // If 'user' is null, show the AuthPage.
  return user ? (
    <MainWebsite user={user} handleSignOut={handleSignOut} />
  ) : (
    <LoginPage/>
    
  );
}

export default App;
