import { create } from "zustand";
import { User } from "firebase/auth";

type AuthUser = {
  uid: string;
  email: string | null;
  idToken?: string; // short-lived
  refreshToken?: string; // long-lived
  offline?: boolean; // mark when logged in offline
};

type AuthState = {
  user: AuthUser | null;
  loading: boolean;
  online: boolean;
  setUser: (user: AuthState["user"]) => void;
  setLoading: (loading: boolean) => void;
  setOnline: (online: boolean) => void;
  reset: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  online: true,
  setUser: (user) => set({ user }),
  setLoading: (loading) => set({ loading }),
  setOnline: (online) => set({ online }),
  reset: () => set({ user: null, loading: false }),
}));
