import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Settings } from "lucide-react";
import Drawer from "@/components/custom/Drawer";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Eye, EyeOff } from "lucide-react";
import { useAuthStore } from "@/store/AuthContext";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  GetTasks,
  LoginUserOnline,
  LogoutUser,
  SyncUnsyncedRecords,
  OpenFesWebApp,
} from "@/../wailsjs/go/app/App";
import { BrowserOpenURL, EventsOn } from "@/../wailsjs/runtime/runtime";

interface SyncUser {
  email: string | null | undefined;
  password: string | null | undefined;
}

const ProfileBar = () => {
  const [open, setOpen] = useState(false);
  const [logoutDialog, setLogoutDialog] = useState(false);
  const { user, setTasks } = useAuthStore();
  const [syncUser, setSyncUser] = useState<SyncUser>({
    email: useAuthStore.getState().user?.email,
    password: undefined,
  });
  const [showPassword, setShowPassword] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [openSyncDialog, setOpenSyncDialog] = useState(false);
  const [syncErrorMsg, setSyncErrorMsg] = useState("");
  const [logoutLoading, setLogoutLoading] = useState(false);
  const [logoutSyncError, setLogoutSyncError] = useState("");
  const [logoutSyncFailDialog, setLogoutSyncFailDialog] = useState(false);

  const { online, synced, setSynced, reset } = useAuthStore();
  const navigate = useNavigate();

  // THEME TOGGLE STATE
  const [dark, setDark] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem("theme") === "dark";
  });

  // Keep theme in sync with <html> and localStorage
  useEffect(() => {
    if (dark) {
      document.documentElement.classList.add("dark");
      localStorage.setItem("theme", "dark");
    } else {
      document.documentElement.classList.remove("dark");
      localStorage.setItem("theme", "light");
    }
  }, [dark]);

  const hasSubscribed = useRef(false);

  useEffect(() => {
    if (!EventsOn) return;
    if (hasSubscribed.current) return;
    hasSubscribed.current = true;

    const off = EventsOn("open_fesapp", () => {
      BrowserOpenURL("fesapp://");
    });

    return () => {
      off();
      hasSubscribed.current = false;
    };
  }, []);

  useEffect(() => {
    if (!EventsOn) return;

    const off = EventsOn("tasks-sync-warning", (message: string) => {
      setOpenSyncDialog(true);
      setSyncErrorMsg(
        `Task sync warning: ${
          typeof message === "string" ? message : String(message)
        }`,
      );
    });

    return () => off();
  }, []);

  const finishLogout = () => {
    const currentTheme = localStorage.getItem("theme");

    localStorage.clear(); // clears user, tokens, etc.

    if (currentTheme) {
      localStorage.setItem("theme", currentTheme);
    }
    try {
      // clear local client state BEFORE navigation
      localStorage.removeItem("user");
      localStorage.removeItem("authToken");

      // ensure your auth store reset runs synchronously
      if (reset) reset();
    } catch (err) {
      console.warn("Cleanup failed:", err);
    } finally {
      // First try client-side navigation
      try {
        navigate("/login", { replace: true });
      } catch (e) {
        console.warn("navigate('/login') failed:", e);
      }

      // Hard fallback: force a full-page replace after a short delay so we never end up stuck
      setTimeout(() => {
        if (window.location.pathname !== "/login") {
          window.location.replace("/login");
        }
      }, 300);
    }
  };

  const getErrorMessage = (error: unknown): string => {
    if (typeof error === "string") return error;
    if (error instanceof Error) return error.message;
    try {
      return JSON.stringify(error);
    } catch {
      return "Sync failed while logging out.";
    }
  };

  const handleLogout = async () => {
    if (logoutLoading) return;

    setLogoutLoading(true);
    setLogoutDialog(false);
    setLogoutSyncError("");
    setLogoutSyncFailDialog(false);

    try {
      await LogoutUser();
      finishLogout();
    } catch (error) {
      console.error("Logout sync failed:", error);
      setLogoutDialog(false);
      setLogoutSyncError(getErrorMessage(error));
      setLogoutSyncFailDialog(true);
    } finally {
      setLogoutLoading(false);
    }
  };

  const handleLogoutWithoutSync = () => {
    setLogoutSyncFailDialog(false);
    finishLogout();
  };

  async function launchFesApp() {
    try {
      await OpenFesWebApp();
    } catch (e) {
      console.error(e);
    }
  }

  async function syncAccount(e?: React.MouseEvent) {
    if (e) e.preventDefault();

    setSyncErrorMsg("");
    setSyncLoading(true);

    if (!online) {
      setSyncLoading(false);
      setOpenSyncDialog(true);
      setSyncErrorMsg("No internet connection. Please go online to sync.");
      return;
    }

    setOpenSyncDialog(false);

    try {
      if (!online) console.log("no internet connection!");
      console.log("Logging in");
      const attendance = await LoginUserOnline(
        syncUser.email || "",
        syncUser.password || "",
      );
      console.log("Auth Response: ", attendance);

      useAuthStore.getState().setUser({
        uid: attendance.UserID,
        email: syncUser.email || "",
        offline: !online,
      });

      const user = useAuthStore.getState().user;
      if (user) {
        localStorage.setItem("user", JSON.stringify(user));
        await SyncUnsyncedRecords();
        setSynced(true);
      }

      const fetchedTasks: any = await GetTasks(false);
      if (fetchedTasks) setTasks(fetchedTasks);
    } catch (error: any) {
      console.error(error);
      setOpenSyncDialog(true);
      setSyncErrorMsg(error);
    } finally {
      setSyncLoading(false);
    }
  }

  useEffect(() => {
    console.log("user updated", user);
  }, [user]);

  return (
    <div className="sticky bottom-0 z-20">
      {/* Local Mode Banner */}
      {!synced && online && (
        <div className="w-full bg-amber-100 dark:bg-amber-900/40 text-xs sm:text-sm flex items-center justify-center gap-2 px-4 py-1 border-t border-amber-200 dark:border-amber-800">
          <span className="inline-block h-2 w-3 rounded-full bg-amber-500" />
          <span className="font-medium text-amber-900 dark:text-amber-100">
            Local Mode - some data is stored offline. Tap{" "}
            <span className="font-semibold">Sync</span> in the footer when
            you&apos;re ready.
          </span>
        </div>
      )}

      {/* Sync Dialog */}
      <Dialog open={openSyncDialog} onOpenChange={setOpenSyncDialog}>
        <form>
          <DialogContent className="sm:max-w-[420px] rounded-2xl bg-white dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-800 shadow-xl">
            <DialogHeader className="space-y-2">
              <DialogTitle className="text-lg sm:text-xl font-semibold text-neutral-900 dark:text-neutral-50">
                Sync your progress
              </DialogTitle>
              <DialogDescription className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
                Upload your latest tracked time to the server so nothing gets
                lost. You can review synced entries in your sync history.
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-4 py-2">
              {/* Email */}
              <div className="grid gap-2">
                <Label
                  htmlFor="sync-email"
                  className="text-sm font-medium text-neutral-800 dark:text-neutral-200"
                >
                  Email
                </Label>
                <Input
                  id="sync-email"
                  type="email"
                  placeholder="name.firstencoding@gmail.com"
                  value={syncUser.email || ""}
                  onChange={(e) =>
                    setSyncUser((prev) => ({
                      ...prev,
                      email: e.target.value,
                    }))
                  }
                  className="rounded-lg bg-white dark:bg-neutral-900 text-sm text-neutral-900 dark:text-neutral-50 placeholder:text-neutral-400 dark:placeholder:text-neutral-500 border border-neutral-300 dark:border-neutral-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:border-emerald-500"
                />
              </div>

              {/* Password */}
              <div className="grid gap-2">
                <Label
                  htmlFor="sync-password"
                  className="text-sm font-medium text-neutral-800 dark:text-neutral-200"
                >
                  Password
                </Label>

                <div className="relative">
                  <Input
                    id="sync-password"
                    type={showPassword ? "text" : "password"}
                    placeholder="••••••••"
                    value={syncUser.password || ""}
                    onChange={(e) =>
                      setSyncUser((prev) => ({
                        ...prev,
                        password: e.target.value,
                      }))
                    }
                    className="rounded-lg bg-white dark:bg-neutral-900 text-sm text-neutral-900 dark:text-neutral-50 placeholder:text-neutral-400 dark:placeholder:text-neutral-500 border border-neutral-300 dark:border-neutral-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:border-emerald-500 pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword((prev) => !prev)}
                    className="absolute inset-y-0 right-0 flex items-center pr-3 text-neutral-500 dark:text-neutral-300 hover:text-neutral-700 dark:hover:text-neutral-100 cursor-pointer"
                    aria-label={
                      showPassword ? "Hide password" : "Show password"
                    }
                  >
                    {showPassword ? (
                      <EyeOff className="w-4 h-4" />
                    ) : (
                      <Eye className="w-4 h-4" />
                    )}
                  </button>
                </div>
              </div>

              {syncErrorMsg && (
                <div className="text-xs sm:text-sm text-red-600 dark:text-red-400 text-center">
                  {syncErrorMsg}
                </div>
              )}
            </div>

            <DialogFooter className="mt-2 flex gap-2 justify-end">
              <DialogClose asChild>
                <Button
                  type="button"
                  variant="outline"
                  className="cursor-pointer rounded-lg border border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-200 bg-white dark:bg-neutral-900 hover:bg-neutral-50 dark:hover:bg-neutral-800"
                >
                  Cancel
                </Button>
              </DialogClose>

              <Button
                type="submit"
                onClick={syncAccount}
                disabled={syncLoading}
                className="
            rounded-lg
            bg-emerald-600 hover:bg-emerald-700
            text-white text-sm font-semibold
            px-4 py-2.5
            cursor-pointer
            disabled:opacity-70 disabled:cursor-not-allowed
          "
              >
                {syncLoading ? "Syncing..." : "Sync now"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </form>
      </Dialog>

      {/* Footer bar */}
      <div className="flex px-4 sm:px-6 py-3 sm:py-4 border-t border-neutral-200 dark:border-neutral-800 bg-white/90 dark:bg-neutral-900/90 backdrop-blur-md slight-shadow w-full justify-between items-center">
        {/* Left side – user info */}
        <div className="flex items-center gap-3 sm:gap-4">
          <button
            onClick={launchFesApp}
            className={
              "cursor-pointer active:opacity-70 transition-all duration-300"
            }
          >
            <img
              // src="//cf9237271c7c9759e747853c3b817950.cdn.bubble.io/f1764841309776x100054280396554060/jp.png"
              src={
                user?.photo ||
                "https://www.shutterstock.com/image-vector/profile-default-avatar-icon-user-600nw-2463844171.jpg"
              }
              className={`rounded-full border-2 object-cover object-top ${
                online ? "border-emerald-500" : "border-neutral-500"
              } select-none`}
              style={{ height: 48, width: 48 }}
            />
          </button>

          <div className="flex flex-col leading-tight">
            <span
              className={`relative pl-3 text-sm font-medium before:content-[''] before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:w-2 before:h-2 before:rounded-full ${
                online
                  ? "before:bg-emerald-500 text-emerald-700 dark:text-emerald-400"
                  : "before:bg-neutral-500 text-neutral-600 dark:text-neutral-400"
              }`}
            >
              {online ? "Online" : "Offline"}
            </span>
            <span className="text-[10px] text-neutral-500 dark:text-neutral-400 tracking-wide">
              Time Tracker · First Encoding
            </span>
          </div>
        </div>

        {/* Right side – Sync + Settings */}
        <div className="flex items-center gap-2 sm:gap-3">
          {online && !synced && (
            <button
              onClick={() => setOpenSyncDialog(true)}
              disabled={syncLoading}
              className="inline-flex justify-center items-center gap-1 min-w-[72px] px-4 py-2 text-xs font-medium rounded-full shadow disabled:opacity-60 cursor-pointer transition bg-emerald-600 text-white hover:bg-emerald-700"
              title="Open sync dialog"
            >
              {syncLoading ? "..." : "Sync"}
            </button>
          )}
          {online && synced && (
            <span className="inline-flex justify-center items-center min-w-[72px] px-4 py-2 text-xs font-medium rounded-full bg-emerald-100 text-emerald-700">
              Synced
            </span>
          )}

          <div className="relative flex items-center group">
            <button
              onClick={() => setOpen(true)}
              className="rounded-full border-2 border-emerald-500/80 p-2.5 sm:p-3 hover:bg-transparent dark:hover:transparent transition cursor-pointer"
            >
              <Settings className="w-5 h-5 sm:w-6 sm:h-6 text-emerald-600 dark:text-emerald-400 transition-transform duration-500 group-hover:rotate-180" />
            </button>

            <span className="absolute bottom-11 sm:bottom-12 left-1/2 -translate-x-1/2 bg-black/80 text-white text-[10px] sm:text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 pointer-events-none transition">
              Settings
            </span>
          </div>
        </div>

        {/* Drawer */}
        <Drawer open={open} onClose={() => setOpen(false)}>
          <div className="w-full bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50 rounded-2xl min-h-[220px] max-h-[90vh] flex flex-col">
            {/* Header */}
            <div className="flex items-center justify-between px-5 pt-4 pb-3 border-b border-neutral-200 dark:border-neutral-800">
              <h2 className="text-lg font-semibold">Settings</h2>
              <button
                onClick={() => setOpen(false)}
                className="text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 cursor-pointer text-xl leading-none"
                aria-label="Close settings"
              >
                ✕
              </button>
            </div>

            {/* Scrollable body */}
            <div className="px-5 pb-5 pt-3 overflow-y-auto flex-1">
              <div className="flex flex-col gap-3 sm:gap-4">
                <Link to="/account">
                  <button className="w-full p-3 rounded-lg border border-gray-300 dark:border-neutral-700 hover:bg-emerald-50 dark:hover:bg-neutral-800 transition cursor-pointer text-center font-semibold text-emerald-600 dark:text-emerald-400 text-sm">
                    Account
                  </button>
                </Link>
                <Link to="/notifications">
                  <button className="w-full p-3 rounded-lg border border-gray-300 dark:border-neutral-700 hover:bg-emerald-50 dark:hover:bg-neutral-800 transition cursor-pointer text-center font-semibold text-emerald-600 dark:text-emerald-400 text-sm">
                    Notifications
                  </button>
                </Link>
                <Link to="/activity-logs">
                  <button className="w-full p-3 rounded-lg border border-gray-300 dark:border-neutral-700 hover:bg-emerald-50 dark:hover:bg-neutral-800 transition cursor-pointer text-center font-semibold text-emerald-600 dark:text-emerald-400 text-sm">
                    Activity Logs
                  </button>
                </Link>

                <button
                  onClick={() => setLogoutDialog(true)}
                  className="w-full p-3 rounded-lg border border-gray-300 dark:border-neutral-700 hover:bg-emerald-50 dark:hover:bg-neutral-800 transition cursor-pointer text-center font-semibold text-red-600 dark:text-red-400 text-sm"
                >
                  Logout
                </button>
              </div>

              {/* Theme section with toggle */}
              <div className="mt-6 pt-4 border-t border-neutral-200 dark:border-neutral-800">
                <div className="flex items-center justify-between">
                  <div className="flex flex-col">
                    <span className="text-xs font-medium text-neutral-600 dark:text-neutral-300">
                      Theme
                    </span>
                    <span className="text-[11px] text-neutral-400 dark:text-neutral-500">
                      Toggle between light and dark mode.
                    </span>
                  </div>
                  <Switch
                    checked={dark}
                    onCheckedChange={setDark}
                    className="cursor-pointer"
                  />
                </div>
              </div>
            </div>
          </div>
        </Drawer>

        {/* Logout dialog */}
        <Dialog open={logoutDialog} onOpenChange={setLogoutDialog}>
          <DialogContent className="sm:max-w-md bg-white dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-800">
            <DialogHeader>
              <DialogTitle className="text-neutral-900 dark:text-neutral-50">
                Confirm Logout
              </DialogTitle>
            </DialogHeader>
            <p className="text-sm text-neutral-600 dark:text-neutral-300">
              Logging out will end your current shift and stop tracking time. Do
              you want to continue?
            </p>
            <DialogFooter className="flex gap-3 justify-end">
              <Button
                variant="outline"
                disabled={logoutLoading}
                onClick={() => setLogoutDialog(false)}
                className="cursor-pointer border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-200"
              >
                Cancel
              </Button>
              <Button
                disabled={logoutLoading}
                onClick={handleLogout}
                className="bg-red-600 hover:bg-red-700 text-white cursor-pointer"
              >
                {logoutLoading ? "Logging out..." : "Logout"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={logoutSyncFailDialog} onOpenChange={setLogoutSyncFailDialog}>
          <DialogContent className="sm:max-w-md bg-white dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-800">
            <DialogHeader>
              <DialogTitle className="text-neutral-900 dark:text-neutral-50">
                Logout Sync Failed
              </DialogTitle>
              <DialogDescription className="text-sm text-neutral-600 dark:text-neutral-300">
                We could not sync your logout session to the server.
              </DialogDescription>
            </DialogHeader>

            {logoutSyncError && (
              <p className="text-xs sm:text-sm text-red-600 dark:text-red-400">
                {logoutSyncError}
              </p>
            )}

            <DialogFooter className="flex gap-2 justify-end">
              <Button
                variant="outline"
                onClick={() => setLogoutSyncFailDialog(false)}
                disabled={logoutLoading}
                className="cursor-pointer border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-200"
              >
                Cancel
              </Button>
              <Button
                variant="outline"
                onClick={handleLogout}
                disabled={logoutLoading}
                className="cursor-pointer border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-200"
              >
                {logoutLoading ? "Retrying..." : "Retry Sync & Logout"}
              </Button>
              <Button
                onClick={handleLogoutWithoutSync}
                className="bg-red-600 hover:bg-red-700 text-white cursor-pointer"
              >
                Logout Without Sync
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  );
};

export default ProfileBar;
