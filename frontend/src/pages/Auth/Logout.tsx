import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { LogoutUser } from "@/../wailsjs/go/app/App";
import { auth } from "@/firebase";
import { signOut } from "firebase/auth";
import { useAuthStore } from "@/store/AuthContext";

const Logout = () => {
  const navigate = useNavigate();
  const { reset } = useAuthStore();

  useEffect(() => {
    async function removeCredentials() {
      // Clear tokens or user session
      const user = localStorage.getItem("user");
      await LogoutUser(user);
      localStorage.removeItem("authToken");
      localStorage.removeItem("user");
      reset();

      // Redirect to login page
      navigate("/login");
    }

    removeCredentials();
  }, [navigate]);

  return null;
};

export default Logout;
