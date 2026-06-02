import { useEffect } from "react";
import "./App.scss";
import "./styles/style.scss";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Home from "./pages/Main/Home";
import Login from "./pages/Auth/Login";
import Task from "./pages/Main/Task";
import AccountSettings from "@/pages/Account/AccountSettings";
import Notifications from "./pages/Main/Notifications";
import ActivityLogs from "./pages/Main/ActivityLogs";
import Logout from "./pages/Auth/Logout";

import { CheckForUpdates, DownloadAndInstall, StopTracking } from "../wailsjs/go/app/App";
import { EventsOn } from "../wailsjs/runtime";
import { useOnlineStatus } from "./hooks/useOnlineStatus";
import { useAuthStore } from "./store/AuthContext";
import OfflineLogin from "./pages/Auth/OfflineLogin";
import Landing from "./pages/Landing";

interface ScreenshotProps {
  date: number;
  pictures: string[];
}

interface UpdateInfo {
  exists: boolean;
  version: string;
  downloadUrl: string;
}

function App() {
  useOnlineStatus();

  const { taskID } = useAuthStore();
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

    handleUpdateCheck();
  }, [enableReleaseUpdates]); // Runs on startup; update checks are opt-in via env flag.

  useEffect(() => {
    if (!EventsOn) return;

    console.log("SHUTDOWN EVENT HANDLER LOADED");
    // TODO this is just temporary measures to ensure logging out of account means task end
    const off = EventsOn("app_shutdown", async () => {
      console.log("SHUTTING DOWN EVENT RECEIVED!");
      if (taskID) {
        try {
          await StopTracking(taskID);
          console.log("SHUTDOWN LOGGED OFF TRACKER");
        } catch (error) {
          console.error("Failed to stop tracking on shutdown:", error);
        }
      }
    });

    return () => {
      off();
    };
  }, [taskID]);

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/home" element={<Home />} />
        <Route path="/login" element={<Login />} />
        <Route path="/offline-login" element={<OfflineLogin />} />
        <Route path="/task/:id" element={<Task />} />
        <Route path="/account" element={<AccountSettings />} />
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
