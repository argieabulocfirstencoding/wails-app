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

import { CheckForUpdates } from "../wailsjs/go/app/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { useFirebaseAuth } from "./hooks/useFirebaseAuth";
import { useOnlineStatus } from "./hooks/useOnlineStatus";
import OfflineLogin from "./pages/Auth/OfflineLogin";
import Landing from "./pages/Landing";
import { updateErrorMessage, useUpdateStore } from "./store/updateStore";
import DesktopUpdateDialog from "./components/custom/DesktopUpdateDialog";

function App() {
  useFirebaseAuth();
  useOnlineStatus();

  const setChecking = useUpdateStore((state) => state.setChecking);
  const setUpdate = useUpdateStore((state) => state.setUpdate);
  const setProgress = useUpdateStore((state) => state.setProgress);
  const setError = useUpdateStore((state) => state.setError);

  useEffect(() => {
    const checkForUpdate = async () => {
      if (useUpdateStore.getState().downloading) return;
      setChecking(true);
      setError(null);
      try {
        const info = await CheckForUpdates();
        setUpdate(info.available ? info : null);
      } catch (err) {
        setError(updateErrorMessage(err, "Unable to check for updates."));
      } finally {
        setChecking(false);
      }
    };

    // App is mounted above the router, so this check and its dialog are available
    // on the login screen as well as after authentication.
    void checkForUpdate();
    const intervalId = window.setInterval(() => {
      void checkForUpdate();
    }, 120_000);
    const unsubscribe = EventsOn("update:download-progress", (progress: number) => setProgress(progress));

    return () => {
      window.clearInterval(intervalId);
      unsubscribe();
    };
  }, [setChecking, setError, setProgress, setUpdate]);

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
