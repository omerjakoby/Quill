import {auth} from '../services/firebaseConfig';
import { GoogleAuthProvider, signInWithPopup } from "firebase/auth";
import "../css/login.css"; 
import logo from "../assets/logo-full-white-removebg-preview.png";
import LetterGlitch from '../assets/LetterGlitch';

// A simple component for the Google 'G' logo
const GoogleIcon = () => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="18"
    height="18"
    viewBox="0 0 48 48"
  >
    <path
      fill="#FFC107"
      d="M43.611 20.083H42V20H24v8h11.303c-1.649 4.657-6.08 8-11.303 8c-6.627 0-12-5.373-12-12s5.373-12 12-12c3.059 0 5.842 1.154 7.961 3.039L38.802 9.92C34.553 6.08 29.613 4 24 4C12.955 4 4 12.955 4 24s8.955 20 20 20s20-8.955 20-20c0-1.341-.138-2.65-.389-3.917z"
    />
    <path
      fill="#FF3D00"
      d="M6.306 14.691l6.571 4.819C14.655 15.108 18.961 12 24 12c3.059 0 5.842 1.154 7.961 3.039L38.802 9.92C34.553 6.08 29.613 4 24 4C16.318 4 9.656 8.337 6.306 14.691z"
    />
    <path
      fill="#4CAF50"
      d="M24 44c5.166 0 9.86-1.977 13.409-5.192l-6.19-5.238C29.211 35.091 26.715 36 24 36c-5.222 0-9.618-3.226-11.283-7.582l-6.522 5.025C9.505 39.556 16.227 44 24 44z"
    />
    <path
      fill="#1976D2"
      d="M43.611 20.083H24v8h11.303c-.792 2.237-2.231 4.166-4.087 5.571l6.19 5.238C42.012 36.49 44 30.617 44 24c0-1.341-.138-2.65-.389-3.917z"
    />
  </svg>
);

function LoginPage() {
  // This function is called when the button is clicked
  const handleGoogleSignIn = async () => {
    const provider = new GoogleAuthProvider();
    try {
      await signInWithPopup(auth, provider);
      // If login is successful, the listener in App.jsx will automatically
      // handle the "transfer" to the main website.
    } catch (error) {
      console.error("Authentication error:", error.message);
    }
  };

  return (
    <div className="background-container">
      <div className="glitch-fullscreen-background">
        <LetterGlitch
          glitchSpeed={50}
          centerVignette={true}
          outerVignette={false}
          smooth={true}
        />
      </div>
      <div className="auth-page-container">
      <img src={logo} alt="Quill Logo" className="Login-logo"></img>
      <h1>Welcome to Quill</h1>
      <p>The modern mail transfer protocol</p>
      <button className="google-signin-button" onClick={handleGoogleSignIn}>
        <GoogleIcon />
        <span>continue with Google</span>
      </button>
    </div>
  </div>
    
  );
}

export default LoginPage;