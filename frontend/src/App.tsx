import { useEffect } from "react";
import "./App.scss";
import "./styles/style.scss";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Home from "./pages/Main/Home";
import Login from "./pages/Auth/Login";
import Task from "./pages/Main/Task";
import AccountSettings from "@/pages/Account/AccountSettings";
import ClientAccounts from "@/pages/Account/ClientAccounts";
import Notifications from "./pages/Main/Notifications";
import ActivityLogs from "./pages/Main/ActivityLogs";
import Logout from "./pages/Auth/Logout";

import { CheckForUpdates, DownloadAndInstall } from "../wailsjs/go/app/App";
import { useFirebaseAuth } from "./hooks/useFirebaseAuth";
import { useOnlineStatus } from "./hooks/useOnlineStatus";
import OfflineLogin from "./pages/Auth/OfflineLogin";
import Landing from "./pages/Landing";
import { releaseFlags } from "./config/release";

interface UpdateInfo {
  exists: boolean;
  version: string;
  downloadUrl: string;
}

function App() {
  useFirebaseAuth();
  useOnlineStatus();

  const enableReleaseUpdates = import.meta.env.VITE_ENABLE_RELEASE_UPDATES === "true";

  useEffect(() => {
    if (!enableReleaseUpdates) {
      return;
    }

    const handleUpdateCheck = async () => {
      try {
        const info: UpdateInfo = await CheckForUpdates();

        if (info.exists) {
          // You could replace this with a nice Tailwind/Radix UI Modal
          const userConfirmed = window.confirm(
            `A new version (${info.version}) is available. Would you like to update now?`,
          );

          if (userConfirmed) {
            console.log("Starting download...");
            await DownloadAndInstall(info.downloadUrl);
          }
        }
      } catch (err) {
        console.error("Failed to check for updates:", err);
      }
    };

    const timerId = window.setTimeout(() => {
      handleUpdateCheck();
    }, releaseFlags.updateCheckDelayMs);

    return () => {
      window.clearTimeout(timerId);
    };
  }, [enableReleaseUpdates]);

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/home" element={<Home />} />
        <Route path="/login" element={<Login />} />
        <Route path="/offline-login" element={<OfflineLogin />} />
        <Route path="/task/:id" element={<Task />} />
        <Route path="/account" element={<AccountSettings />} />
        <Route path="/client-accounts" element={<ClientAccounts />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/activity-logs" element={<ActivityLogs />} />
        <Route path="/logout" element={<Logout />} />

        {/* Not Found */}
        <Route path="*" element={<Login />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
