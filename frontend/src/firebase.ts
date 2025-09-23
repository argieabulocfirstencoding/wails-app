// Import the functions you need from the SDKs you need
import { initializeApp } from "firebase/app";
import { getAuth } from "firebase/auth";
// TODO: Add SDKs for Firebase products that you want to use
// https://firebase.google.com/docs/web/setup#available-libraries

// Your web app's Firebase configuration
const firebaseConfig = {
  apiKey: "AIzaSyDujSDNKM0cvXX9IEUp4qHOy3d8D8GSSBg",
  authDomain: "fesapp-4892c.firebaseapp.com",
  projectId: "fesapp-4892c",
  storageBucket: "fesapp-4892c.firebasestorage.app",
  messagingSenderId: "443582948751",
  appId: "1:443582948751:web:36bfad15493384d3d717fa",
};

// Initialize Firebase
const app = initializeApp(firebaseConfig);
const auth = getAuth(app);

export { app, auth };
