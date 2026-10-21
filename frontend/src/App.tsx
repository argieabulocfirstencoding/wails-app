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

<<<<<<< HEAD
import { CheckForUpdates, DownloadAndInstall } from "../wailsjs/go/app/App";
=======
import { CheckForUpdates } from "../wailsjs/go/app/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
import { useFirebaseAuth } from "./hooks/useFirebaseAuth";
import { useOnlineStatus } from "./hooks/useOnlineStatus";
import OfflineLogin from "./pages/Auth/OfflineLogin";
import Landing from "./pages/Landing";
<<<<<<< HEAD
import { releaseFlags } from "./config/release";

interface UpdateInfo {
  exists: boolean;
  version: string;
  downloadUrl: string;
}
=======
import { updateErrorMessage, useUpdateStore } from "./store/updateStore";
import DesktopUpdateDialog from "./components/custom/DesktopUpdateDialog";
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef

function App() {
  useFirebaseAuth();
  useOnlineStatus();

<<<<<<< HEAD
  const enableReleaseUpdates = import.meta.env.VITE_ENABLE_RELEASE_UPDATES === "true";

  useEffect(() => {
    if (!enableReleaseUpdates) {
      return;
    }

    const handleUpdateCheck = async () => {
=======
  const setChecking = useUpdateStore((state) => state.setChecking);
  const setUpdate = useUpdateStore((state) => state.setUpdate);
  const setProgress = useUpdateStore((state) => state.setProgress);
  const setError = useUpdateStore((state) => state.setError);

  useEffect(() => {
    const checkForDevelopmentUpdate = async () => {
      setChecking(true);
      setError(null);
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
      try {
        const info = await CheckForUpdates();
        setUpdate(info.available ? info : null);
      } catch (err) {
        setError(updateErrorMessage(err, "Unable to check for updates."));
      } finally {
        setChecking(false);
      }
    };

<<<<<<< HEAD
    const timerId = window.setTimeout(() => {
      handleUpdateCheck();
    }, releaseFlags.updateCheckDelayMs);

    return () => {
      window.clearTimeout(timerId);
    };
  }, [enableReleaseUpdates]);
=======
    // App is mounted above the router, so this check and its dialog are available
    // on the login screen as well as after authentication.
    void checkForDevelopmentUpdate();
    const intervalId = window.setInterval(() => {
      void checkForDevelopmentUpdate();
    }, 120_000);
    const unsubscribe = EventsOn("update:download-progress", (progress: number) => setProgress(progress));

    return () => {
      window.clearInterval(intervalId);
      unsubscribe();
    };
  }, [setChecking, setError, setProgress, setUpdate]);
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef

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
