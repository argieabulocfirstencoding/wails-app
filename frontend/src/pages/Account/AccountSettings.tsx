import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "@/store/AuthContext";
import { Button } from "@/components/ui/button";

const AccountSettings = () => {
  const navigate = useNavigate();
  const storeUser = useAuthStore((s) => s.user);
  const [user, setUser] = useState({
    fullName: "",
    email: "",
    position: "",
    birthdate: "",
    phone: "",
    avatar: "",
  });
  const [dark, setDark] = useState<boolean>(
    () => localStorage.getItem("theme") === "dark"
  );

  useEffect(() => {
    // hydrate from store or localStorage
    if (storeUser) {
      // storeUser shape may not include fullName/avatar in the AuthUser type.
      // cast to any and try common property names (fullName, displayName, name, photoURL, avatar)
      const su: any = storeUser;
      setUser((u) => ({
        ...u,
        email: su.email ?? u.email,
        fullName: (su.fullName ??
          su.displayName ??
          su.name ??
          u.fullName) as string,
        avatar: (su.avatar ?? su.photoURL ?? su.picture ?? "") as string,
      }));
    } else {
      const saved = localStorage.getItem("user");
      if (saved) {
        try {
          const parsed = JSON.parse(saved);
          setUser((u) => ({ ...u, email: parsed.email ?? u.email }));
        } catch {}
      }
    }
  }, [storeUser]);

  useEffect(() => {
    // apply persisted theme on mount
    const isDark = localStorage.getItem("theme") === "dark" || dark;
    document.documentElement.classList.toggle("dark", Boolean(isDark));
    setDark(Boolean(isDark));
  }, []);

  function toggleTheme() {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("theme", next ? "dark" : "light");
  }

  return (
    <div className="min-h-screen p-6 bg-gray-50 dark:bg-neutral-950">
      <div className="max-w-xl mx-auto">
        <button
          onClick={() => navigate(-1)}
          className="text-gray-600 dark:text-gray-300 mb-4"
        >
          &lt; Back
        </button>

        <div className="bg-white dark:bg-neutral-900 rounded-2xl shadow-lg p-6">
          <div className="flex flex-col items-center">
            <div className="w-28 h-28 rounded-full overflow-hidden border-2 border-gray-200 dark:border-gray-700 mb-4">
              <img
                src={user.avatar || "/assets/images/default-avatar.png"}
                alt="avatar"
                className="w-full h-full object-cover"
              />
            </div>

            <h2 className="text-2xl font-extrabold text-gray-900 dark:text-white mb-1">
              Account Settings
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">
              View your personal details
            </p>

            <div className="w-full grid gap-4">
              <div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Full Name
                </p>
                <p className="font-medium text-gray-900 dark:text-gray-100">
                  {user.fullName || "—"}
                </p>
              </div>

              <div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Email Address
                </p>
                <p className="truncate font-medium text-gray-900 dark:text-gray-100">
                  {user.email || "—"}
                </p>
              </div>

              <div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Position
                </p>
                <p className="font-medium text-gray-900 dark:text-gray-100">
                  {user.position || "—"}
                </p>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Birthdate
                  </p>
                  <p className="font-medium text-gray-900 dark:text-gray-100">
                    {user.birthdate || "—"}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Contact Number
                  </p>
                  <p className="font-medium text-gray-900 dark:text-gray-100">
                    {user.phone || "—"}
                  </p>
                </div>
              </div>

              <div className="flex items-center justify-between mt-4">
                <div className="flex gap-2">
                  <Button
                    onClick={() => navigate("/account/edit")}
                    className="bg-green-600 hover:bg-green-700 text-white"
                  >
                    Edit Profile
                  </Button>
                  <Button
                    onClick={() => {
                      // sign out or other action placeholder
                      localStorage.removeItem("user");
                      useAuthStore.getState().setUser(null);
                      navigate("/login");
                    }}
                    className="bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
                  >
                    Sign Out
                  </Button>
                </div>

                <div className="flex items-center gap-3">
                  <span className="text-sm text-gray-600 dark:text-gray-300">
                    {dark ? "Dark" : "Light"}
                  </span>
                  <button
                    onClick={toggleTheme}
                    className="w-10 h-6 rounded-full p-0.5 bg-gray-200 dark:bg-gray-700 relative"
                    aria-label="Toggle theme"
                  >
                    <span
                      className={`block w-4 h-4 rounded-full bg-white shadow transform transition-transform ${
                        dark ? "translate-x-4" : "translate-x-0"
                      }`}
                    />
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AccountSettings;
