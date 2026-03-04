import { useAuthStore } from "@/store/AuthContext";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { GetTotalClientRecords } from "wailsjs/go/app/App";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const fetchTodayClientSummary = async (optionalDate?: Date) => {
  const { attendance, setSummary } = useAuthStore.getState();

  try {
    // Determine which date to use: parameter > attendance.CreatedAt
    const dateToUse =
      optionalDate ??
      (attendance?.CreatedAt ? new Date(attendance.CreatedAt) : null);

    if (!dateToUse) return;

    // Format date to 'yyyy-MM-dd' in Manila timezone
    const dateOnly = dateToUse.toLocaleDateString("en-CA", {
      timeZone: "Asia/Manila",
    });

    // Call backend
    const results = await GetTotalClientRecords(dateOnly);
    console.log("Client summary results:", results);

    // Update Zustand store
    setSummary(results);
  } catch (error) {
    console.error("Error fetching client summary:", error);
  }
};
