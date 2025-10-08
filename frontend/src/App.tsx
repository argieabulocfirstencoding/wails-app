import { useEffect, useState } from "react";
import "./App.scss";
import "./styles/style.scss";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Home from "./pages/Main/Home";
import Login from "./pages/Auth/Login";
import Task from "./pages/Main/Task";
import AccountSettings from "./pages/Main/AccountSettings";
import Notifications from "./pages/Main/Notifications";
import ActivityLogs from "./pages/Main/ActivityLogs";
import Logout from "./pages/Auth/Logout";

import { StopTracking } from "../wailsjs/go/app/App";
import { EventsOn } from "../wailsjs/runtime";
import Signup from "./pages/Auth/Signup";
import { useFirebaseAuth } from "./hooks/useFirebaseAuth";
import { useOnlineStatus } from "./hooks/useOnlineStatus";
import { useAuthStore } from "./store/AuthContext";
import OfflineLogin from "./pages/Auth/OfflineLogin";

interface ScreenshotProps {
  date: number;
  pictures: string[];
}

function App() {
  useFirebaseAuth();
  useOnlineStatus();

  const { taskID } = useAuthStore();

  useEffect(() => {
    console.log("SHUTDOWN EVENT HANDLER LOADED");
    // TODO this is just temporary measures to ensure logging out of account means task end
    const off = EventsOn("app_shutdown", async () => {
      console.log("SHUTTING DOWN EVENT RECEIVED!");
      if (taskID) {
        await StopTracking(taskID);
        console.log("SHUTDOWN LOGGED OFF TRACKER");
      }
    });

    return () => {
      off();
    };
  }, [taskID]);

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login" element={<Login />} />
        <Route path="/offline-login" element={<OfflineLogin />} />
        <Route path="/task/:id" element={<Task />} />
        <Route path="/account-settings" element={<AccountSettings />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/activity-logs" element={<ActivityLogs />} />
        <Route path="/logout" element={<Logout />} />
        <Route path="/signup" element={<Signup />} />

        {/* Not Found */}
        <Route path="*" element={<Login />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
