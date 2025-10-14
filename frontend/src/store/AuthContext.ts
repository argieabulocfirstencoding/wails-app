import { models } from "wailsjs/go/models";
import { create } from "zustand";

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
  taskID: string | null;
  fetchedActiveRecord: boolean;
  synced: boolean;
  tasks: models.ExpTask[];
  setTaskID: (taskID: string | null) => void;
  setTasks: (tasks: models.ExpTask[]) => void;
  setUser: (user: AuthState["user"]) => void;
  setLoading: (loading: boolean) => void;
  setOnline: (online: boolean) => void;
  reset: () => void;
  setFetchedActiveRecord: (record: boolean) => void;
  setSynced: (synced: boolean) => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  online: navigator.onLine,
  taskID: null,
  fetchedActiveRecord: false,
  synced: false,
  tasks: [],
  setTasks: (tasks) => set({ tasks }),
  setTaskID: (taskID) => set({ taskID }),
  setUser: (user) => set({ user }),
  setLoading: (loading) => set({ loading }),
  setOnline: (online) => set({ online }),
  setFetchedActiveRecord: (fetchedActiveRecord) => set({ fetchedActiveRecord }),
  setSynced: (synced) => set({ synced }),
  reset: () =>
    set({
      user: null,
      loading: false,
      online: navigator.onLine,
      taskID: null,
      fetchedActiveRecord: false,
      synced: false,
    }),
}));
