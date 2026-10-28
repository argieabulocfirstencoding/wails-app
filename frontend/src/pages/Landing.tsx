import Loader from "@/components/custom/Loader";
import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { RestoreInterruptedSession } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";

const Landing = () => {
  const navigate = useNavigate();
  useEffect(() => {
    let cancelled = false;

    async function restore() {
      try {
        const recovery = await RestoreInterruptedSession();
        if (cancelled) return;
        if (recovery?.recovered && recovery.user && recovery.attendance) {
          const authStore = useAuthStore.getState();
          authStore.setUser({
            uid: recovery.user.localId,
            email: recovery.user.email,
            name: recovery.user.name,
            position: recovery.user.position,
            birthdate: recovery.user.birthdate,
            phone: recovery.user.phone,
            address: recovery.user.address,
            photo: recovery.user.photo,
            fesid: recovery.user.fesid,
          });
          authStore.setAttendance(recovery.attendance);
          // The backend has already rehydrated the active task before it
          // returns this response. Seed the store now so Home renders it as
          // running even while its normal refresh calls are still in flight.
          authStore.setTaskID(recovery.activeTaskId || null);
          authStore.setFetchedActiveRecord(false);
          authStore.setSynced(true);
          authStore.setRecoveryState(true, false);
          navigate("/home", { replace: true });
          return;
        }
      } catch (error) {
        console.warn("Interrupted-session recovery was unavailable:", error);
      }
      if (!cancelled) navigate("/login", { replace: true });
    }

    restore();
    return () => {
      cancelled = true;
    };
  }, [navigate]);

  return (
    <div className="w-full h-[100vh] dark:bg-neutral-900 flex flex-col items-center justify-center">
      <div className="mb-5">
        <Loader />
      </div>
      <span className="text-black dark:text-white">Loading your FES App</span>
    </div>
  );
};

export default Landing;
