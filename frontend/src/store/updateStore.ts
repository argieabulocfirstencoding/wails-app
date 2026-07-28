import { create } from "zustand";

export type DesktopUpdate = {
  available: boolean;
  channel: string;
  branch: string;
  currentVersion: string;
  latestVersion: string;
  currentCommit: string;
  latestCommit: string;
  installerUrl: string;
  checksumUrl: string;
};

type UpdateState = {
  checking: boolean;
  update: DesktopUpdate | null;
  downloading: boolean;
  progress: number;
  error: string | null;
  setChecking: (checking: boolean) => void;
  setUpdate: (update: DesktopUpdate | null) => void;
  dismiss: () => void;
  setDownloading: (downloading: boolean) => void;
  setProgress: (progress: number) => void;
  setError: (error: string | null) => void;
};

export const useUpdateStore = create<UpdateState>((set) => ({
  checking: false,
  update: null,
  downloading: false,
  progress: 0,
  error: null,
  setChecking: (checking) => set({ checking }),
  setUpdate: (update) => set({ update, error: null }),
  dismiss: () => set({ update: null, error: null }),
  setDownloading: (downloading) =>
    set({ downloading, progress: downloading ? 0 : 100 }),
  setProgress: (progress) => set({ progress: Math.max(0, Math.min(progress, 100)) }),
  setError: (error) => set({ error }),
}));
