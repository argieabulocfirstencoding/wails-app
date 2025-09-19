import { useEffect, useState } from "react";
import logo from "./assets/images/logo-universal.png";
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

import { StartTracking, StopTracking } from "../wailsjs/go/app/App";
import { EventsOn } from "../wailsjs/runtime";

interface ScreenshotProps {
  date: number;
  pictures: string[];
}

function App() {
  useEffect(() => {
    // TODO this is just temporary measures to ensure logging out of account means task end
    const off = EventsOn("app_shutdown", () => {
      const timeStop = Date.now();
      localStorage.setItem("ended", JSON.stringify(timeStop));
      StopTracking();
    });

    return () => {
      off();
    };
  }, []);

  useEffect(() => {
    const off = EventsOn("screenshot", (pictures: string[]) => {
      const now = Date.now();

      const screenshotTaken: ScreenshotProps = { date: now, pictures };
      localStorage.setItem("last_screenshot", JSON.stringify(screenshotTaken));
    });

    return () => {
      off();
    };
  }, []);

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login" element={<Login />} />
        <Route path="/task" element={<Task />} />
        <Route path="/account-settings" element={<AccountSettings />} />
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
