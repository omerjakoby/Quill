// src/services/firebaseConfig.js

// 1. Import the functions you need
import { initializeApp } from "firebase/app";
import { getAuth } from "firebase/auth"; // We need getAuth for logging in

// 2. Your web app's Firebase configuration (This is your correct object)
const firebaseConfig = {
  apiKey: "AIzaSyAwLni-frP5p1dpUuOMPQr8gR0IIVUUxR8",
  authDomain: "quill-mtp.firebaseapp.com",
  projectId: "quill-mtp",
  storageBucket: "quill-mtp.firebasestorage.app",
  messagingSenderId: "771595937562",
  appId: "1:771595937562:web:7ba9bf6e72a0660a6feea3",
  measurementId: "G-ZZ5KLPP37F"
};

// 3. Initialize Firebase
const app = initializeApp(firebaseConfig);

// 4. Initialize and EXPORT the Authentication service
//    The 'export' keyword is what lets other files use it.
export const auth = getAuth(app);