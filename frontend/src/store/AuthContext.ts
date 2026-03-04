import { models, types } from "wailsjs/go/models";
import { create } from "zustand";

type AuthUser = {
  uid: string;
  email: string | null;
  name?: string;
  position?: string;
  birthdate?: string;
  phone?: string;
  address?: string;
  photo?: string;
  fesid?: string;
  idToken?: string; // short-lived
  refreshToken?: string; // long-lived
  offline?: boolean; // mark when logged in offline
};

type AuthState = {
  user: AuthUser | null;
  attendanceSynced: boolean;
  loading: boolean;
  online: boolean;
  taskID: string | null;
  fetchedActiveRecord: boolean;
  synced: boolean;
  tasks: models.ExpTask[];
  summaries: types.TaskSummary[];
  attendance: models.Attendance | undefined;
  setAttendance: (a: models.Attendance) => void;
  setSummary: (summary: types.TaskSummary[]) => void;
  setAttendanceSynced: (synced: boolean) => void;
  setTaskID: (taskID: string | null) => void;
  setTasks: (tasks: models.ExpTask[]) => void;
  setUser: (user: AuthState["user"]) => void;
  setLoading: (loading: boolean) => void;
  setOnline: (online: boolean) => void;
  reset: () => void;
  setFetchedActiveRecord: (record: boolean) => void;
  setSynced: (synced: boolean) => void;
  initializedTask: boolean;
  setInitializedTask: (initialized: boolean) => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  online: navigator.onLine,
  taskID: null,
  fetchedActiveRecord: false,
  synced: false,
  attendanceSynced: false,
  initializedTask: false,
  tasks: [],
  summaries: [],
  attendance: undefined,
  setAttendance: (a) => set({ attendance: a }),
  setSummary: (summaries) => set({ summaries }),
  setAttendanceSynced: (synced) => set({ attendanceSynced: synced }),
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
      attendanceSynced: false,
      initializedTask: false,
    }),
  setInitializedTask: (initializedTask) => set({ initializedTask }),
}));
