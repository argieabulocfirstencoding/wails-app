import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { LogoutUser } from "@/../wailsjs/go/app/App";

const Logout = () => {
  const navigate = useNavigate();

  useEffect(() => {
    async function removeCredentials() {
      // Clear tokens or user session
      const user = localStorage.getItem("user");
      await LogoutUser(user);
      localStorage.removeItem("authToken");
      localStorage.removeItem("user");

      // Redirect to login page
      navigate("/login");
    }

    removeCredentials();
  }, [navigate]);

  return null;
};

export default Logout;
