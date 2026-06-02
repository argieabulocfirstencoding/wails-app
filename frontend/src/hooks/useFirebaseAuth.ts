import { useEffect } from "react";
import { onAuthStateChanged } from "firebase/auth";
import { auth } from "@/firebase";
import { useAuthStore } from "@/store/AuthContext";

export function useFirebaseAuth() {
  const setUser = useAuthStore((s) => s.setUser);
  const setLoading = useAuthStore((s) => s.setLoading);

  useEffect(() => {
    const unsubscribe = onAuthStateChanged(auth, (user) => {
      // The app's login flow is backend-driven. Only hydrate Firebase users if
      // one actually exists; do not clear the local session here.
      if (user) {
        setUser({
          uid: user.uid,
          email: user.email,
          name: user.displayName ?? undefined,
          offline: false,
        });
      }
      setLoading(false);
    });

    return () => unsubscribe();
  }, [setUser, setLoading]);
}
