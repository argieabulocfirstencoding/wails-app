import React, { useEffect, useState } from "react";
import { useNavigate, useLocation, Navigate } from "react-router-dom";
import { LogoutUser } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import { X } from "lucide-react";

const Logout = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { reset } = useAuthStore();
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState("");

  // Do NOT auto-open by default — only open when explicitly requested by navigation state or query param
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const stateOpen = (location.state as any)?.open === true;
    const queryOpen =
      new URLSearchParams(location.search).get("open") === "true";
    if (stateOpen || queryOpen) setOpen(true);
  }, [location]);

  async function handleLogout() {
    setLoading(true);
    setErrorMsg("");

    try {
      await LogoutUser();
      setOpen(false);
    } catch (err) {
      console.warn("LogoutUser failed:", err);
      setErrorMsg(
        err instanceof Error
          ? err.message
          : "Logout failed. Your local session is still active.",
      );
      setLoading(false);
      setOpen(true);
      return;
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
      setLoading(false);

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
  }

  function handleCancel() {
    setOpen(false);
    navigate(-1);
  }

  // If modal is closed, redirect to /login to avoid rendering a blank page route
  if (!open) return <Navigate to="/login" replace />;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      aria-label="Confirm Logout"
    >
      {/* backdrop */}
      <div
        className="absolute inset-0 bg-black/65 backdrop-blur-sm"
        onClick={handleCancel}
      />

      {/* modal */}
      <div className="relative z-10 w-full max-w-md mx-4">
        <div className="bg-neutral-900 border border-gray-800 rounded-xl shadow-2xl p-6">
          <div className="flex items-start justify-between">
            <div>
              <h3 className="text-lg font-semibold text-gray-100">
                Confirm Logout
              </h3>
              <p className="mt-2 text-sm text-gray-300">
                Logging out means ending your shift. Do you wish to proceed?
              </p>
              {errorMsg ? (
                <p className="mt-3 text-sm text-red-400">{errorMsg}</p>
              ) : null}
            </div>

            <button
              onClick={handleCancel}
              className="ml-4 text-gray-400 hover:text-gray-200 rounded-full p-1"
              aria-label="Close"
              title="Close"
              type="button"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          <div className="mt-6 space-y-3">
            <button
              onClick={handleLogout}
              disabled={loading}
              type="button"
              className="
                w-full
                bg-green-600 hover:bg-green-700
                text-white font-medium
                py-3 rounded-lg
                transition-colors disabled:opacity-70
                focus:outline-none focus:ring-2 focus:ring-green-400/60
              "
            >
              {loading ? "Logging out..." : "Logout"}
            </button>

            <button
              onClick={handleCancel}
              type="button"
              className="
                w-full
                bg-transparent
                border border-green-700
                text-green-200 dark:text-green-300
                py-3 rounded-lg
                hover:bg-green-900/10 dark:hover:bg-green-700/10
                transition-colors
              "
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Logout;
