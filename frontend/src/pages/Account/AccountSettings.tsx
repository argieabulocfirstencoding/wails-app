import React, { useEffect, useState } from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import { useAuthStore } from "@/store/AuthContext";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import { User, Mail, Phone, MapPin, Briefcase, Moon, Sun } from "lucide-react";

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

const DEFAULT_USER: AccountUser = {
  fullName: "Arjun P. Rosell",
  email: "arjunrosell.firstencoding@gmail.com",
  position: "Web Developer",
  birthdate: "March 10, 2000",
  phone: "09234567890",
  address: "Compostela, Cebu",
  employmentStatus: "Probationary",
  avatar:
    "https://lh3.googleusercontent.com/a/ACg8ocJWYobBkgAVfMMKK1NZZoAlx3Zk9xr1ONbhYI7bJL-W6cpYSNE=s288-c-no",
};

const AccountSettings = () => {
  const storeUser = useAuthStore((s) => s.user);

  const { user } = useAuthStore();

  // const [user, setUser] = useState<AccountUser>(DEFAULT_USER);

  const [dark, setDark] = useState<boolean>(
    () => localStorage.getItem("theme") === "dark"
  );

  // Hydrate user from store/localStorage
  // useEffect(() => {
  //   if (storeUser) {
  //     const su: any = storeUser;
  //     setUser((prev) => ({
  //       ...prev,
  //       email: su.email ?? prev.email,
  //       fullName:
  //         (su.fullName ?? su.displayName ?? su.name ?? prev.fullName) ||
  //         prev.fullName,
  //       avatar: (su.avatar ??
  //         su.photoURL ??
  //         su.picture ??
  //         prev.avatar) as string,
  //       position: su.position ?? prev.position,
  //     }));
  //   } else {
  //     const saved = localStorage.getItem("user");
  //     if (saved) {
  //       try {
  //         const parsed = JSON.parse(saved);
  //         setUser((prev) => ({
  //           ...prev,
  //           email: parsed.email ?? prev.email,
  //         }));
  //       } catch {
  //         // ignore parse errors
  //       }
  //     }
  //   }
  // }, [storeUser]);

  // Apply theme on mount
  useEffect(() => {
    const isDark = localStorage.getItem("theme") === "dark" || dark;
    document.documentElement.classList.toggle("dark", Boolean(isDark));
    setDark(Boolean(isDark));
  }, []);

  function handleThemeChange(next: boolean) {
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("theme", next ? "dark" : "light");
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
                src={user?.photo || "/assets/images/default-avatar.png"}
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
              {/* <div>
                <p className="text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-500">
                  Employment type
                </p>
                <p className="text-xs font-semibold text-neutral-900 dark:text-neutral-50">
                  {user.employmentStatus}
                </p>
              </div> */}
            </div>

            {/* Edit only (no sign out here) */}
            {/* <div className="pt-3 border-t border-neutral-200 dark:border-neutral-800 flex justify-start">
              <Button
                type="button"
                className="px-4 py-2 text-sm font-semibold bg-emerald-600 rounded-full hover:bg-emerald-700 text-white cursor-pointer"
              >
                Edit profile
              </Button>
            </div> */}
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

        <AppFooterInfo className="mt-4" />
      </div>
    </SettingsLayout>
  );
};

export default AccountSettings;
