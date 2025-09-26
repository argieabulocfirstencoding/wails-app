import { useAuthStore } from "@/store/AuthContext";
import { useEffect } from "react";

export function useOnlineStatus() {
  const setOnline = useAuthStore((s) => s.setOnline);

  useEffect(() => {
    function handleOnline() {
      setOnline(true);
    }
    function handleOffline() {
      setOnline(false);
    }

    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);

    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, [setOnline]);
}
