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

import { EventsOn } from "../wailsjs/runtime/runtime";
import { useOnlineStatus } from "./hooks/useOnlineStatus";
import OfflineLogin from "./pages/Auth/OfflineLogin";
import Landing from "./pages/Landing";
import { useUpdateStore } from "./store/updateStore";
import DesktopUpdateDialog from "./components/custom/DesktopUpdateDialog";

function App() {
  useOnlineStatus();

  const setProgress = useUpdateStore((state) => state.setProgress);

  useEffect(() => {
    const unsubscribe = EventsOn("update:download-progress", (progress: number) => setProgress(progress));

    return () => {
      unsubscribe();
    };
  }, [setProgress]);

  return (
    <BrowserRouter>
      <DesktopUpdateDialog />
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
