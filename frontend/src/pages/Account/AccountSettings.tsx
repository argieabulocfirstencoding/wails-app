import React, { useEffect, useState } from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import { useAuthStore } from "@/store/AuthContext";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import {
  User,
  Mail,
  Phone,
  MapPin,
  Briefcase,
  Moon,
  Sun,
  Folder,
  GitBranch,
  RefreshCw,
  PackageCheck,
} from "lucide-react";
import { GetFolderSize, ClearScreenshots, GetBuildInfo, CheckForUpdates } from "@/../wailsjs/go/app/App";
import { releaseFlags } from "@/config/release";
import { updateErrorMessage, useUpdateStore } from "@/store/updateStore";

type AccountUser = {
  fullName: string;
  email: string;
  position: string;
  birthdate: string;
  phone: string;
  address: string;
  employmentStatus: string;
  avatar: string;
};

type BuildInfo = {
  version: string;
  environment: string;
  updateChannel: string;
  updateBranch: string;
  commitSha: string;
  buildNumber: number;
  updatesEnabled: boolean;
};

const AccountSettings = () => {
  const { user } = useAuthStore();

  const [size, setSize] = useState<number>(0);
  const [buildInfo, setBuildInfo] = useState<BuildInfo | null>(null);
  const [updateCheckMessage, setUpdateCheckMessage] = useState<string | null>(null);
  const checkingForUpdate = useUpdateStore((state) => state.checking);
  const setCheckingForUpdate = useUpdateStore((state) => state.setChecking);
  const setAvailableUpdate = useUpdateStore((state) => state.setUpdate);
  const setUpdateError = useUpdateStore((state) => state.setError);

  // const [user, setUser] = useState<AccountUser>(DEFAULT_USER);

  const [dark, setDark] = useState<boolean>(
    () => localStorage.getItem("theme") === "dark",
  );

  useEffect(() => {
    fetchScreenshotSize();
    void fetchBuildInfo();
  }, []);

  // Apply theme on mount
  useEffect(() => {
    const isDark = localStorage.getItem("theme") === "dark" || dark;
    document.documentElement.classList.toggle("dark", Boolean(isDark));
    setDark(Boolean(isDark));
  }, []);

  async function fetchScreenshotSize() {
    try {
      const result = await GetFolderSize();
      if (result) {
        setSize(result);
      }
    } catch (error) {
      console.error("error on fetching screenshot size: ", error);
    }
  }

  async function fetchBuildInfo() {
    try {
      setBuildInfo(await GetBuildInfo());
    } catch (error) {
      console.error("error on fetching build information: ", error);
    }
  }

  async function checkForUpdates() {
    setCheckingForUpdate(true);
    setUpdateError(null);
    setUpdateCheckMessage(null);
    try {
      const update = await CheckForUpdates();
      setAvailableUpdate(update.available ? update : null);
      if (!update.available) {
        setUpdateCheckMessage("No newer update is available.");
      }
    } catch (error) {
      const message = updateErrorMessage(error, "Unable to check for updates.");
      setUpdateError(message);
      setUpdateCheckMessage(message);
    } finally {
      setCheckingForUpdate(false);
    }
  }

  async function clearScreenshotStorage() {
    if (size === 0) return;

    try {
      await ClearScreenshots();
      const newSize = await GetFolderSize();
      setSize(newSize);
    } catch (error) {
      console.error("Error clearing screenshots:", error);
    }
  }

  function handleThemeChange(next: boolean) {
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("theme", next ? "dark" : "light");
  }

  function byteConverter(bytes: number): string {
    if (bytes >= 1024 ** 3) {
      return (bytes / 1024 ** 3).toFixed(2) + " GB";
    } else if (bytes >= 1024 ** 2) {
      return (bytes / 1024 ** 2).toFixed(2) + " MB";
    } else if (bytes >= 1024) {
      return (bytes / 1024).toFixed(2) + " KB";
    } else {
      return bytes + " B";
    }
  }

  return (
    <SettingsLayout>
      <div className="max-w-md mx-auto space-y-6">
        {/* Page title */}
        <header className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
            Account
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
            View your profile details and appearance settings.
          </p>
        </header>

        {/* Profile card */}
        <section
          className="
            rounded-2xl
            border border-neutral-200 dark:border-neutral-800
            bg-white dark:bg-neutral-900
            overflow-hidden
          "
        >
          {/* Label */}
          <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800 flex items-center justify-between">
            <span className="text-[11px] font-semibold tracking-[0.14em] uppercase text-neutral-500 dark:text-neutral-500">
              Profile
            </span>
            <User className="w-4 h-4 text-neutral-400 dark:text-neutral-500" />
          </div>

          {/* Avatar + name + email */}
          <div className="px-6 pt-5 pb-4 flex items-center gap-4">
            <div className="w-16 h-16 rounded-full overflow-hidden border border-neutral-200 dark:border-neutral-700 flex-shrink-0">
              <img
                src={user?.photo || "https://www.shutterstock.com/image-vector/profile-default-avatar-icon-user-600nw-2463844171.jpg"}
                alt="Profile avatar"
                className="w-full h-full object-cover"
              />
            </div>

            <div className="space-y-1 min-w-0">
              <p className="text-sm font-semibold text-neutral-900 dark:text-neutral-50 truncate">
                {user?.name}
              </p>
              <p className="text-xs text-neutral-600 dark:text-neutral-400 flex items-center gap-1 truncate">
                <Mail className="w-3 h-3 text-neutral-400 dark:text-neutral-500" />
                <span className="truncate">{user?.email}</span>
              </p>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 flex items-center gap-1 truncate">
                <Briefcase className="w-3 h-3 text-neutral-400 dark:text-neutral-500" />
                <span>{user?.position}</span>
              </p>
            </div>
          </div>

          {/* Details */}
          <div className="px-6 pb-5 space-y-4">
            {/* Birthdate + contact */}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-500">
                  Birthdate
                </p>
                <p className="text-xs font-semibold text-neutral-900 dark:text-neutral-50">
                  {user?.birthdate}
                </p>
              </div>
              <div>
                <p className="text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-500">
                  Contact
                </p>
                <p className="text-xs font-semibold text-neutral-900 dark:text-neutral-50 flex items-center gap-1">
                  <Phone className="w-3.5 h-3.5 text-neutral-400 dark:text-neutral-500" />
                  <span>{user?.phone || "—"}</span>
                </p>
              </div>
            </div>

            {/* Address + employment type */}
            <div className="flex">
              <div>
                <p className="text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-500">
                  Address
                </p>
                <p className="text-xs font-semibold text-neutral-900 dark:text-neutral-50 flex items-center gap-1">
                  <MapPin className="w-3.5 h-3.5 text-neutral-400 dark:text-neutral-500" />
                  <span>{user?.address}</span>
                </p>
              </div>
            </div>
          </div>
        </section>

        {/* Preferences card */}
        <section
          className="
            rounded-2xl
            border border-neutral-200 dark:border-neutral-800
            bg-white dark:bg-neutral-900
            overflow-hidden
          "
        >
          <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800">
            <span className="text-[11px] font-semibold tracking-[0.14em] uppercase text-neutral-500 dark:text-neutral-500">
              Preferences
            </span>
          </div>

          {/* Theme row with Switch */}
          <div className="px-6 py-4 flex items-center justify-between gap-4">
            <div className="flex items-start gap-3 flex-1">
              <div className="mt-0.5 rounded-full bg-neutral-100 dark:bg-neutral-800 p-1.5">
                {dark ? (
                  <Moon className="w-4 h-4 text-neutral-700 dark:text-neutral-200" />
                ) : (
                  <Sun className="w-4 h-4 text-amber-500" />
                )}
              </div>
              <div className="space-y-0.5">
                <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                  Theme switcher
                </p>
                <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                  Switch between light and dark mode for the app interface.
                </p>
              </div>
            </div>

            <Switch
              checked={dark}
              onCheckedChange={handleThemeChange}
              className="flex-shrink-0 cursor-pointer"
            />
          </div>
        </section>

        {/* Preferences card */}
        <section
          className="
            rounded-2xl
            border border-neutral-200 dark:border-neutral-800
            bg-white dark:bg-neutral-900
            overflow-hidden
          "
        >
          <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800">
            <span className="text-[11px] font-semibold tracking-[0.14em] uppercase text-neutral-500 dark:text-neutral-500">
              Features
            </span>
          </div>

          <div className="px-6 py-4 flex items-center justify-between gap-4">
            <div className="flex items-start gap-3 flex-1">
              <div className="mt-0.5 rounded-full bg-neutral-100 dark:bg-neutral-800 p-1.5">
                <Folder className="w-4 h-4 text-neutral-700 dark:text-neutral-200" />
              </div>
              <div className="space-y-0.5">
                <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                  Screenshots Storage
                </p>
                <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                  {releaseFlags.screenshotsEnabled
                    ? byteConverter(size || 0)
                    : `Disabled for this release. Stored data: ${byteConverter(size || 0)}`}
                </p>
              </div>
            </div>

            <Button
              className="text-white font-bold bg-red-500 cursor-pointer hover:bg-red-400 active:bg-red-700"
              onClick={clearScreenshotStorage}
              disabled={size === 0}
            >
              Clear
            </Button>
          </div>
        </section>

        <section
          className="rounded-2xl border border-neutral-200 bg-white overflow-hidden dark:border-neutral-800 dark:bg-neutral-900"
        >
          <div className="flex items-center justify-between border-b border-neutral-200 px-6 py-3 dark:border-neutral-800">
            <span className="text-[11px] font-semibold tracking-[0.14em] uppercase text-neutral-500 dark:text-neutral-500">
              Build & updates
            </span>
            <PackageCheck className="size-4 text-emerald-600 dark:text-emerald-400" />
          </div>

          <div className="space-y-4 px-6 py-4">
            <div className="grid grid-cols-2 gap-x-4 gap-y-3 text-xs">
              <div>
                <p className="text-[10px] font-semibold tracking-wide text-neutral-500 uppercase">Version</p>
                <p className="mt-1 font-medium text-neutral-900 dark:text-neutral-50">{buildInfo?.version ?? "Loading…"}</p>
              </div>
              <div>
                <p className="text-[10px] font-semibold tracking-wide text-neutral-500 uppercase">Channel</p>
                <p className="mt-1 font-medium capitalize text-neutral-900 dark:text-neutral-50">{buildInfo?.updateChannel ?? "Loading…"}</p>
              </div>
              <div>
                <p className="text-[10px] font-semibold tracking-wide text-neutral-500 uppercase">Branch</p>
                <p className="mt-1 flex items-center gap-1 font-mono text-[11px] text-neutral-700 dark:text-neutral-300"><GitBranch className="size-3" />{buildInfo?.updateBranch ?? "—"}</p>
              </div>
              <div>
                <p className="text-[10px] font-semibold tracking-wide text-neutral-500 uppercase">Commit</p>
                <p className="mt-1 font-mono text-[11px] text-neutral-700 dark:text-neutral-300">{buildInfo?.commitSha?.slice(0, 7) ?? "—"}</p>
              </div>
            </div>

            <div className="flex items-center justify-between gap-3 border-t border-neutral-100 pt-4 dark:border-neutral-800">
              <p className="text-xs leading-relaxed text-neutral-500 dark:text-neutral-400">
                {buildInfo?.updatesEnabled ? "Update checking is enabled." : "Updates are disabled for this build."}
              </p>
              <Button variant="outline" className="shrink-0 gap-2" onClick={checkForUpdates} disabled={!buildInfo?.updatesEnabled || checkingForUpdate}>
                <RefreshCw className={`size-3.5 ${checkingForUpdate ? "animate-spin" : ""}`} />
                {checkingForUpdate ? "Checking" : "Check for updates"}
              </Button>
            </div>
            {updateCheckMessage && (
              <p className="rounded-lg bg-neutral-50 px-3 py-2 text-xs text-neutral-600 dark:bg-neutral-800/70 dark:text-neutral-300">
                {updateCheckMessage}
              </p>
            )}
          </div>
        </section>

        <AppFooterInfo className="mt-4" />
      </div>
    </SettingsLayout>
  );
};

export default AccountSettings;
