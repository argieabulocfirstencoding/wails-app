import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { LogoutUser } from "@/../wailsjs/go/app/App";
import { auth } from "@/firebase";
import { signOut } from "firebase/auth";
import { useAuthStore } from "@/store/AuthContext";

const Logout = () => {
  const navigate = useNavigate();
  const { synced, reset } = useAuthStore();

  useEffect(() => {
    async function removeCredentials() {
      // Clear tokens or user session
      try {
        await LogoutUser();
        localStorage.removeItem("authToken");
        localStorage.removeItem("user");
        reset();

        console.log("Synced: ", synced)

        // Redirect to login page
        navigate("/login");
      } catch (error) {
        console.log("ERROR LOGGING OUT: ", error);
      }
    }

    removeCredentials();
  }, [navigate]);

  return null;
};

export default Logout;
