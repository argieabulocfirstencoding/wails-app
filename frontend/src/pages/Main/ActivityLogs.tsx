import { useCallback, useEffect, useState } from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import {
  AlertTriangle,
  CheckCircle2,
  History,
  LogIn,
  LogOut,
  RotateCcw,
  UserCircle2,
  XCircle,
} from "lucide-react";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import { models } from "@/../wailsjs/go/models";
import {
  GetActivityLogRecord,
  GetTotalClientRecords,
<<<<<<< HEAD
  SyncAttendanceByID,
  SyncUnsyncedTaskRecord,
=======
  ResyncTaskRecord,
  SyncAttendanceByID,
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
} from "@/../wailsjs/go/app/App";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/store/AuthContext";
import { Calendar22 } from "@/components/ui/calendar22";
import { secondsToHHMM } from "@/lib/utils";
<<<<<<< HEAD
=======
import { EventsOn } from "@/../wailsjs/runtime/runtime";
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
import { formatInManila, supportsManilaTimeZone, toManilaDateOnly } from "@/lib/timezone";

const RESYNC_WINDOW_MS = 48 * 60 * 60 * 1000;

const toDateOnly = (input: Date): string => {
  return toManilaDateOnly(input);
};

const logActivityUI = (
  level: "info" | "warn" | "error",
  message: string,
  context?: Record<string, unknown>,
) => {
  const payload = {
    message,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    timestamp: new Date().toISOString(),
    ...context,
  };

  if (level === "error") {
    console.error("[ActivityLogs]", payload);
    return;
  }

  if (level === "warn") {
    console.warn("[ActivityLogs]", payload);
    return;
  }
<<<<<<< HEAD

  console.info("[ActivityLogs]", payload);
};

const parseDateTime = (value: unknown): Date | null => {
  if (!value) return null;
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value;
  }
  if (typeof value === "number") {
    const numericDate = new Date(value);
    return Number.isNaN(numericDate.getTime()) ? null : numericDate;
  }
  if (typeof value !== "string") return null;

  const trimmed = value.trim();
  if (!trimmed) return null;

  const normalized = trimmed.includes("T")
    ? trimmed
    : trimmed.replace(" ", "T");
  const hasTimezone = /(?:Z|[+-]\d{2}:?\d{2})$/i.test(normalized);
  const withTimezone = hasTimezone ? normalized : `${normalized}+08:00`;

  const parsed = new Date(withTimezone);
  if (!Number.isNaN(parsed.getTime())) return parsed;

  const fallback = new Date(trimmed);
  return Number.isNaN(fallback.getTime()) ? null : fallback;
};

const formatTimestamp = (value: unknown) => {
  const parsed = parseDateTime(value);
  if (!parsed) return { date: "Invalid date", time: "" };

  const date = formatInManila(parsed, "en-US", {
    month: "2-digit",
    day: "2-digit",
    year: "numeric",
  });

  const time = formatInManila(parsed, "en-US", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: true,
  });

  return { date, time };
};

const isSynced = (syncStatus?: string, syncedAt?: unknown): boolean => {
  return syncStatus?.toLowerCase() === "synced" && !!parseDateTime(syncedAt);
};

const statusLabel = (syncStatus?: string, syncedAt?: unknown): string => {
  if (isSynced(syncStatus, syncedAt)) return "Synced";
  if (syncStatus?.toLowerCase() === "pending") return "Pending";
  return "Unsynced";
};

const getPendingScreenshotCount = (
  record?: models.TaskRecord | null,
): number => {
  const count = record?.PendingScreenshotCount ?? 0;
  return count > 0 ? count : 0;
};

const canResyncAttendanceByTime = (value: unknown): boolean => {
  const parsed = parseDateTime(value);
  if (!parsed) return false;

  const ageMs = Date.now() - parsed.getTime();
  if (ageMs < 0) return true;

  return ageMs < RESYNC_WINDOW_MS;
};

const renderEventIcon = (event: string) => {
  const lower = event.toLowerCase();
  if (lower.includes("logged in")) {
    return (
      <div className="mt-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/40 p-1.5">
        <LogIn className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
      </div>
    );
  }
  if (lower.includes("logged out")) {
    return (
      <div className="mt-0.5 rounded-full bg-rose-100 dark:bg-rose-900/40 p-1.5">
        <LogOut className="w-3.5 h-3.5 text-rose-600 dark:text-rose-400" />
      </div>
    );
  }
  return (
    <div className="mt-0.5 rounded-full bg-neutral-200 dark:bg-neutral-800 p-1.5">
      <History className="w-3.5 h-3.5 text-neutral-700 dark:text-neutral-300" />
    </div>
  );
};

const getErrorMessage = (error: unknown): string => {
  if (typeof error === "string") return error;
  if (error instanceof Error) return error.message;
  if (error && typeof error === "object" && "message" in error) {
    const value = (error as { message?: unknown }).message;
    if (typeof value === "string") return value;
  }

  try {
    return JSON.stringify(error);
  } catch {
    return "Sync failed. Please retry.";
  }
};

const ActivityLogs = () => {
  const [activities, setActivities] = useState<models.ActivityLogRecord[]>([]);
  const [date, setDate] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string>("");
  const [loadWarning, setLoadWarning] = useState<string>("");
  const [syncingKeys, setSyncingKeys] = useState<Record<string, boolean>>({});
  const [syncErrorByKey, setSyncErrorByKey] = useState<Record<string, string>>(
    {},
  );
  const [syncConfirmOpen, setSyncConfirmOpen] = useState(false);
  const [pendingSync, setPendingSync] = useState<{
    type: "task" | "attendance";
    id: number;
    key: string;
    label: string;
  } | null>(null);
  const { summaries, setSummary, tasks, online } = useAuthStore();

  const loadActivityData = useCallback(
    async (targetDate: Date) => {
      setLoading(true);
      setLoadError("");
      setLoadWarning("");
      try {
          const dateOnly = toDateOnly(targetDate);
          logActivityUI("info", "Loading activity data", {
            targetDateISO: targetDate.toISOString(),
            queryDate: dateOnly,
            manilaTimezoneSupported: supportsManilaTimeZone(),
          });

        const [activitiesResult, summaryResult] = await Promise.allSettled([
          GetActivityLogRecord(dateOnly),
          GetTotalClientRecords(dateOnly),
        ]);

        if (activitiesResult.status === "fulfilled") {
          setActivities(activitiesResult.value ?? []);
          logActivityUI("info", "Activity records loaded", {
            queryDate: dateOnly,
            count: activitiesResult.value?.length ?? 0,
          });
        } else {
          const errorMessage = getErrorMessage(activitiesResult.reason);
          setActivities([]);
          setLoadError(errorMessage);
          logActivityUI("error", "Failed to load activity records", {
            queryDate: dateOnly,
            error: errorMessage,
            raw: activitiesResult.reason,
          });
        }

        if (summaryResult.status === "fulfilled") {
          setSummary(summaryResult.value ?? []);
          logActivityUI("info", "Summary records loaded", {
            queryDate: dateOnly,
            count: summaryResult.value?.length ?? 0,
          });
        } else {
          const summaryError = getErrorMessage(summaryResult.reason);
          setSummary([]);
          setLoadWarning(
            `Activity summary could not be loaded: ${summaryError}`,
          );
          logActivityUI("warn", "Failed to load summary records", {
            queryDate: dateOnly,
            error: summaryError,
            raw: summaryResult.reason,
          });
        }
      } catch (error) {
        const errorMessage = getErrorMessage(error);
        setActivities([]);
        setSummary([]);
        setLoadError(errorMessage);
        logActivityUI("error", "Unexpected error loading activity page", {
          error: errorMessage,
          raw: error,
        });
      } finally {
        setLoading(false);
      }
    },
    [setSummary],
  );

  useEffect(() => {
    loadActivityData(date);
  }, [date, loadActivityData]);

  const setSyncing = (key: string, syncing: boolean) => {
    setSyncingKeys((prev) => ({ ...prev, [key]: syncing }));
  };

=======

  console.info("[ActivityLogs]", payload);
};

const parseDateTime = (value: unknown): Date | null => {
  if (!value) return null;
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value;
  }
  if (typeof value === "number") {
    const numericDate = new Date(value);
    return Number.isNaN(numericDate.getTime()) ? null : numericDate;
  }
  if (typeof value !== "string") return null;

  const trimmed = value.trim();
  if (!trimmed) return null;

  const normalized = trimmed.includes("T")
    ? trimmed
    : trimmed.replace(" ", "T");
  const hasTimezone = /(?:Z|[+-]\d{2}:?\d{2})$/i.test(normalized);
  const withTimezone = hasTimezone ? normalized : `${normalized}+08:00`;

  const parsed = new Date(withTimezone);
  if (!Number.isNaN(parsed.getTime())) return parsed;

  const fallback = new Date(trimmed);
  return Number.isNaN(fallback.getTime()) ? null : fallback;
};

const formatTimestamp = (value: unknown) => {
  const parsed = parseDateTime(value);
  if (!parsed) return { date: "Invalid date", time: "" };

  const date = formatInManila(parsed, "en-US", {
    month: "2-digit",
    day: "2-digit",
    year: "numeric",
  });

  const time = formatInManila(parsed, "en-US", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: true,
  });

  return { date, time };
};

const isSynced = (syncStatus?: string, syncedAt?: unknown): boolean => {
  return syncStatus?.toLowerCase() === "synced" && !!parseDateTime(syncedAt);
};

const statusLabel = (syncStatus?: string, syncedAt?: unknown): string => {
  if (isSynced(syncStatus, syncedAt)) return "Synced";
  if (syncStatus?.toLowerCase() === "remote_missing") {
    return "Missing in Bubble";
  }
  if (syncStatus?.toLowerCase() === "identity_conflict") return "Review required";
  if (syncStatus?.toLowerCase() === "pending") return "Pending";
  return "Unsynced";
};

const getPendingScreenshotCount = (
  record?: models.TaskRecord | null,
): number => {
  const count = record?.PendingScreenshotCount ?? 0;
  return count > 0 ? count : 0;
};

const canResyncAttendanceByTime = (value: unknown): boolean => {
  const parsed = parseDateTime(value);
  if (!parsed) return false;

  const ageMs = Date.now() - parsed.getTime();
  if (ageMs < 0) return true;

  return ageMs < RESYNC_WINDOW_MS;
};

const renderEventIcon = (event: string) => {
  const lower = event.toLowerCase();
  if (lower.includes("logged in")) {
    return (
      <div className="mt-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/40 p-1.5">
        <LogIn className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
      </div>
    );
  }
  if (lower.includes("logged out")) {
    return (
      <div className="mt-0.5 rounded-full bg-rose-100 dark:bg-rose-900/40 p-1.5">
        <LogOut className="w-3.5 h-3.5 text-rose-600 dark:text-rose-400" />
      </div>
    );
  }
  return (
    <div className="mt-0.5 rounded-full bg-neutral-200 dark:bg-neutral-800 p-1.5">
      <History className="w-3.5 h-3.5 text-neutral-700 dark:text-neutral-300" />
    </div>
  );
};

const getErrorMessage = (error: unknown): string => {
  if (typeof error === "string") return error;
  if (error instanceof Error) return error.message;
  if (error && typeof error === "object" && "message" in error) {
    const value = (error as { message?: unknown }).message;
    if (typeof value === "string") return value;
  }

  try {
    return JSON.stringify(error);
  } catch {
    return "Sync failed. Please retry.";
  }
};

const ActivityLogs = () => {
  const [activities, setActivities] = useState<models.ActivityLogRecord[]>([]);
  const [date, setDate] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string>("");
  const [loadWarning, setLoadWarning] = useState<string>("");
  const [syncingKeys, setSyncingKeys] = useState<Record<string, boolean>>({});
  const [syncErrorByKey, setSyncErrorByKey] = useState<Record<string, string>>(
    {},
  );
  const [syncConfirmOpen, setSyncConfirmOpen] = useState(false);
  const [pendingSync, setPendingSync] = useState<{
    type: "task" | "attendance";
    id: number;
    key: string;
    label: string;
  } | null>(null);
  const { summaries, setSummary, tasks, online } = useAuthStore();

  const loadActivityData = useCallback(
    async (targetDate: Date) => {
      setLoading(true);
      setLoadError("");
      setLoadWarning("");
      try {
          const dateOnly = toDateOnly(targetDate);
          logActivityUI("info", "Loading activity data", {
            targetDateISO: targetDate.toISOString(),
            queryDate: dateOnly,
            manilaTimezoneSupported: supportsManilaTimeZone(),
          });

        const [activitiesResult, summaryResult] = await Promise.allSettled([
          GetActivityLogRecord(dateOnly),
          GetTotalClientRecords(dateOnly),
        ]);

        if (activitiesResult.status === "fulfilled") {
          setActivities(activitiesResult.value ?? []);
          logActivityUI("info", "Activity records loaded", {
            queryDate: dateOnly,
            count: activitiesResult.value?.length ?? 0,
          });
        } else {
          const errorMessage = getErrorMessage(activitiesResult.reason);
          setActivities([]);
          setLoadError(errorMessage);
          logActivityUI("error", "Failed to load activity records", {
            queryDate: dateOnly,
            error: errorMessage,
            raw: activitiesResult.reason,
          });
        }

        if (summaryResult.status === "fulfilled") {
          setSummary(summaryResult.value ?? []);
          logActivityUI("info", "Summary records loaded", {
            queryDate: dateOnly,
            count: summaryResult.value?.length ?? 0,
          });
        } else {
          const summaryError = getErrorMessage(summaryResult.reason);
          setSummary([]);
          setLoadWarning(
            `Activity summary could not be loaded: ${summaryError}`,
          );
          logActivityUI("warn", "Failed to load summary records", {
            queryDate: dateOnly,
            error: summaryError,
            raw: summaryResult.reason,
          });
        }
      } catch (error) {
        const errorMessage = getErrorMessage(error);
        setActivities([]);
        setSummary([]);
        setLoadError(errorMessage);
        logActivityUI("error", "Unexpected error loading activity page", {
          error: errorMessage,
          raw: error,
        });
      } finally {
        setLoading(false);
      }
    },
    [setSummary],
  );

  useEffect(() => {
    loadActivityData(date);
  }, [date, loadActivityData]);

  useEffect(() => {
    if (!EventsOn) return;
    const off = EventsOn("browser-records-synced", () => {
      void loadActivityData(date);
    });

    return () => off();
  }, [date, loadActivityData]);
  const setSyncing = (key: string, syncing: boolean) => {
    setSyncingKeys((prev) => ({ ...prev, [key]: syncing }));
  };

>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
  const clearSyncError = (key: string) => {
    setSyncErrorByKey((prev) => ({ ...prev, [key]: "" }));
  };

  const setSyncError = (key: string, message: string) => {
    setSyncErrorByKey((prev) => ({ ...prev, [key]: message }));
  };

  const syncTaskRecord = async (recordID: number, key: string) => {
    if (!online) {
      setSyncError(key, "Cannot sync while offline.");
      logActivityUI("warn", "Blocked task resync while offline", {
        recordID,
        key,
      });
      return;
    }

    setSyncing(key, true);
    clearSyncError(key);
    try {
      logActivityUI("info", "Task resync requested", { recordID, key });
<<<<<<< HEAD
      await SyncUnsyncedTaskRecord(recordID);
=======
      await ResyncTaskRecord(recordID);
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
      await loadActivityData(date);
      logActivityUI("info", "Task resync completed", { recordID, key });
    } catch (error) {
      const errorMessage = getErrorMessage(error);
      setSyncError(key, errorMessage);
      logActivityUI("error", "Task resync failed", {
        recordID,
        key,
        error: errorMessage,
        raw: error,
      });
    } finally {
      setSyncing(key, false);
    }
  };

  const syncAttendanceRecord = async (attendanceID: number, key: string) => {
    if (!online) {
      setSyncError(key, "Cannot sync while offline.");
      logActivityUI("warn", "Blocked attendance resync while offline", {
        attendanceID,
        key,
      });
      return;
    }

    setSyncing(key, true);
    clearSyncError(key);
    try {
      logActivityUI("info", "Attendance resync requested", {
        attendanceID,
        key,
      });
      await SyncAttendanceByID(attendanceID);
      await loadActivityData(date);
      logActivityUI("info", "Attendance resync completed", {
        attendanceID,
        key,
      });
    } catch (error) {
      const errorMessage = getErrorMessage(error);
      setSyncError(key, errorMessage);
      logActivityUI("error", "Attendance resync failed", {
        attendanceID,
        key,
        error: errorMessage,
        raw: error,
      });
    } finally {
      setSyncing(key, false);
    }
  };

  const findTaskByID = (id: string) => {
    return tasks.find((task) => task.taskId === id) ?? null;
  };

  const requestSync = (
    type: "task" | "attendance",
    id: number,
    key: string,
    label: string,
  ) => {
    setPendingSync({ type, id, key, label });
    setSyncConfirmOpen(true);
  };

  const confirmSync = async () => {
    if (!pendingSync) return;

    setSyncConfirmOpen(false);
    const { type, id, key } = pendingSync;
    setPendingSync(null);

    if (type === "task") {
      await syncTaskRecord(id, key);
      return;
    }

    await syncAttendanceRecord(id, key);
  };

  return (
    <SettingsLayout>
<<<<<<< HEAD
      <div className="max-w-md mx-auto space-y-6">
=======
      <div className="relative max-w-md mx-auto space-y-6">
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
        <header className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
            Activity Logs
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
            Here you can manage your activity logs.
          </p>
        </header>

<<<<<<< HEAD
        <div className="absolute top-3 z-999 right-4 flex justify-end items-end">
=======
        <div className="fixed top-3 z-999 right-4 flex justify-end items-end">
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
          <Calendar22 setDate={setDate} date={date} />
        </div>

        {summaries && summaries.length > 0 && (
          <section className="rounded-2xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 overflow-hidden">
            <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800 flex items-center justify-between">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
                Client
              </span>
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
                Total Time
              </span>
            </div>
            {summaries.map((summary, idx) => (
              <div
                key={summary.TaskID + " " + idx}
                className="px-4 py-4 flex items-start justify-between gap-4"
              >
                <div className="flex items-start gap-3">
                  <UserCircle2 />
                  <div className="space-y-0.5">
                    <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                      {summary.TaskName}
                    </p>
                    {summary.Padagan && (
                      <p className="text-[8px] text-red-500 dark:text-red-400 leading-relaxed">
                        Padagan
                      </p>
                    )}
                  </div>
                </div>
                <div className="flex flex-col items-end w-32 gap-1">
                  <span className="text-[11px] text-neutral-500 dark:text-neutral-400">
                    <div className="flex flex-col items-end leading-tight">
                      <span className="text-neutral-700 dark:text-neutral-300">
                        Total Time:
                      </span>
                      <span className="text-neutral-500 dark:text-neutral-400">
                        {secondsToHHMM(summary.TotalDuration)} hrs
                      </span>
                    </div>
                  </span>
                </div>
              </div>
            ))}
          </section>
        )}

        <section className="rounded-2xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 overflow-hidden">
          <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800 flex items-center justify-between">
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
              Recent Activity
            </span>
          </div>

          {loadError && (
            <div className="px-6 py-3 border-b border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-950/20 text-[11px] text-rose-700 dark:text-rose-300">
              Failed to load activity logs: {loadError}
            </div>
          )}
          {loadWarning && !loadError && (
            <div className="px-6 py-3 border-b border-amber-200 dark:border-amber-900/40 bg-amber-50 dark:bg-amber-950/20 text-[11px] text-amber-700 dark:text-amber-300">
              {loadWarning}
            </div>
          )}

          <div className="divide-y divide-neutral-200 dark:divide-neutral-800">
            {loading ? (
              <div className="px-6 py-6 text-sm text-neutral-500 dark:text-neutral-400">
                Loading activity logs...
              </div>
            ) : activities.length === 0 ? (
              <div className="px-6 py-6 text-sm text-neutral-500 dark:text-neutral-400">
                No activity logs yet.
              </div>
            ) : (
              activities.map((activity, idx) => {
                const attendanceLoginKey = `attendance-login-${activity.attendance.ID}`;
                const attendanceLogoutKey = `attendance-logout-${activity.attendance.ID}`;
                const loginResyncAllowed = canResyncAttendanceByTime(
                  activity.attendance.TimeIn,
                );
                const logoutResyncAllowed = activity.attendance.TimeOut
                  ? canResyncAttendanceByTime(activity.attendance.TimeOut)
                  : false;

                return (
                  <div key={activity.attendance.ID + "-" + idx}>
                    <ActivityBanner
                      label="Logged In"
                      description="FES App Tracker session started"
                      isoTime={activity.attendance.TimeIn}
                      syncStatus={activity.attendance.SyncStatus}
                      syncedAt={activity.attendance.SyncedAt}
                      syncing={!!syncingKeys[attendanceLoginKey]}
                      syncError={syncErrorByKey[attendanceLoginKey]}
                      resyncAllowed={loginResyncAllowed}
                      resyncBlockedReason="Login resync is only available within 2 days."
                      onSync={() =>
                        requestSync(
                          "attendance",
                          activity.attendance.ID,
                          attendanceLoginKey,
                          "Logged In",
                        )
                      }
                    />

                    {activity.records.map((record) => {
                      const task = findTaskByID(record.TaskID);
                      const key = `record-${record.ID}`;
                      const pendingScreenshotCount =
                        getPendingScreenshotCount(record);
                      const taskName = task?.title
                        ? `${task.title.slice(0, 50)}${
                            task.title.length > 50 ? "..." : ""
                          }`
                        : "Task";

                      return (
                        <ActivityBanner
                          key={key}
                          label="Task Recorded"
                          description={`${taskName} task has been recorded`}
                          isoTime={record.StartedAt}
                          syncStatus={record.SyncStatus}
                          syncedAt={record.SyncedAt}
                          syncing={!!syncingKeys[key]}
<<<<<<< HEAD
                          syncError={syncErrorByKey[key]}
=======
                          syncError={
                            syncErrorByKey[key] ||
                            (record.SyncStatus?.toLowerCase() === "remote_missing"
                              ? record.LastError
                              : "")
                          }
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
                          pendingScreenshotCount={pendingScreenshotCount}
                          statusLabelOverride={
                            pendingScreenshotCount > 0
                              ? "Screenshots Pending"
                              : undefined
                          }
                          statusTone={
                            pendingScreenshotCount > 0 ? "warning" : undefined
                          }
                          resyncAllowed={true}
                          onSync={() =>
                            requestSync("task", record.ID, key, "Task Recorded")
                          }
                        />
                      );
                    })}

                    {activity.attendance.TimeOut && (
                      <ActivityBanner
                        label="Logged Out"
                        description="FES App Tracker session ended"
                        isoTime={activity.attendance.TimeOut}
                        syncStatus={activity.attendance.SyncStatus}
                        syncedAt={activity.attendance.SyncedAt}
                        syncing={!!syncingKeys[attendanceLogoutKey]}
                        syncError={syncErrorByKey[attendanceLogoutKey]}
                        resyncAllowed={logoutResyncAllowed}
                        resyncBlockedReason="Logout resync is only available within 2 days."
                        onSync={() =>
                          requestSync(
                            "attendance",
                            activity.attendance.ID,
                            attendanceLogoutKey,
                            "Logged Out",
                          )
                        }
                      />
                    )}
                  </div>
                );
              })
            )}
          </div>
        </section>

        <footer className="flex items-start gap-2 text-xs text-neutral-500 dark:text-neutral-400">
          <AlertTriangle className="w-3.5 h-3.5 mt-0.5 text-amber-500" />
          <p>
            Activity is based on your attendance sessions. Login and logout
            entries are created whenever you start or stop tracking.
          </p>
        </footer>
      </div>
      <AppFooterInfo className="mt-6" />

      <Dialog
        open={syncConfirmOpen}
        onOpenChange={(open) => {
          setSyncConfirmOpen(open);
          if (!open) {
            setPendingSync(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-[420px] rounded-2xl bg-white dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-800 shadow-xl">
          <DialogHeader className="space-y-2">
            <DialogTitle className="text-lg sm:text-xl font-semibold text-neutral-900 dark:text-neutral-50">
<<<<<<< HEAD
              Confirm Resync
            </DialogTitle>
            <DialogDescription className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
              Resyncing this activity may override what was recorded by another
              tracker for the same session.
=======
              Confirm Resync or Repair
            </DialogTitle>
            <DialogDescription className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
              Resyncing may override what was recorded by another tracker. A record missing in Bubble will create a fresh Bubble productivity record before its final details are synced.
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
            </DialogDescription>
          </DialogHeader>

          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            Activity: {pendingSync?.label ?? "-"}
          </p>

          <DialogFooter className="mt-2 flex gap-2 justify-end">
            <Button
              type="button"
              variant="outline"
              className="cursor-pointer rounded-lg border border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-200 bg-white dark:bg-neutral-900 hover:bg-neutral-50 dark:hover:bg-neutral-800"
              onClick={() => {
                setSyncConfirmOpen(false);
                setPendingSync(null);
              }}
            >
              Cancel
            </Button>

            <Button
              type="button"
              onClick={confirmSync}
              className="rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-semibold px-4 py-2.5 cursor-pointer"
            >
<<<<<<< HEAD
              Continue Resync
=======
              Continue
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsLayout>
  );
};

type ActivityBannerProps = {
  label: string;
  description: string;
  isoTime: unknown;
  syncStatus?: string;
  syncedAt?: unknown;
  syncing: boolean;
  syncError?: string;
  pendingScreenshotCount?: number;
  statusLabelOverride?: string;
  statusTone?: "success" | "warning" | "danger";
  resyncAllowed?: boolean;
  resyncBlockedReason?: string;
  onSync: () => void;
};

function ActivityBanner({
  label,
  description,
  isoTime,
  syncStatus,
  syncedAt,
  syncing,
  syncError,
  pendingScreenshotCount = 0,
  statusLabelOverride,
  statusTone,
  resyncAllowed = true,
  resyncBlockedReason = "Resync is no longer available for this entry.",
  onSync,
}: ActivityBannerProps) {
  const eventTime = formatTimestamp(isoTime);
  const syncedTime = formatTimestamp(syncedAt);
  const synced = isSynced(syncStatus, syncedAt);
<<<<<<< HEAD
  const currentStatus = statusLabelOverride ?? statusLabel(syncStatus, syncedAt);
  const currentTone =
    statusTone ?? (synced ? "success" : "danger");
=======
  const remoteMissing = syncStatus?.toLowerCase() === "remote_missing";
  const identityConflict = syncStatus?.toLowerCase() === "identity_conflict";
  const requiresReview = remoteMissing || identityConflict;
  const currentStatus = statusLabelOverride ?? statusLabel(syncStatus, syncedAt);
  const currentTone =
    statusTone ?? (synced ? "success" : requiresReview ? "warning" : "danger");
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
  const pendingNote =
    pendingScreenshotCount > 0
      ? syncing
        ? `Syncing task record and ${pendingScreenshotCount} pending screenshot${
            pendingScreenshotCount === 1 ? "" : "s"
          }...`
        : `${pendingScreenshotCount} screenshot${
            pendingScreenshotCount === 1 ? "" : "s"
          } pending upload`
      : "";

  return (
    <div className="px-4 py-4 flex items-start justify-between gap-4">
      <div className="flex items-start gap-3">
        {renderEventIcon(label)}
        <div className="space-y-0.5">
          <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
            {label}
          </p>
          <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
            {description}
          </p>
          {pendingNote && (
            <p className="text-[10px] text-amber-600 dark:text-amber-400">
              {pendingNote}
            </p>
          )}
          {syncError && (
            <p className="text-[10px] text-rose-500 dark:text-rose-400">
              {syncError}
            </p>
          )}
          {!resyncAllowed && (
            <p className="text-[10px] text-amber-600 dark:text-amber-400">
              {resyncBlockedReason}
            </p>
          )}
<<<<<<< HEAD
=======
          {requiresReview && (
            <p className="text-[10px] text-amber-600 dark:text-amber-400">
              Review the matching Bubble session before resyncing this record.
            </p>
          )}
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
        </div>
      </div>

      <div className="flex flex-col items-end gap-1">
        <div className="flex flex-col items-end leading-tight text-[11px]">
          <span className="text-neutral-700 dark:text-neutral-300">
            {eventTime.date}
          </span>
          <span className="text-neutral-500 dark:text-neutral-400">
            {eventTime.time}
          </span>
        </div>

        <div
          className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium ${
            currentTone === "success"
              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
              : currentTone === "warning"
                ? "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
              : "bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
          }`}
        >
          {currentTone === "success" ? (
            <CheckCircle2 className="w-3 h-3" />
          ) : currentTone === "warning" ? (
            <AlertTriangle className="w-3 h-3" />
          ) : (
            <XCircle className="w-3 h-3" />
          )}
          <span>{currentStatus}</span>
        </div>

        {synced && (
          <div className="text-[9px] text-neutral-500 dark:text-neutral-400 flex flex-col items-end leading-tight">
            <span className="text-emerald-500 dark:text-emerald-400">
              Synced at
            </span>
            <span className="text-neutral-700 dark:text-neutral-300">
              {syncedTime.date}
            </span>
            <span>{syncedTime.time}</span>
          </div>
        )}

        <button
          onClick={onSync}
<<<<<<< HEAD
          disabled={syncing || !resyncAllowed}
          className="inline-flex items-center gap-1 rounded-full border border-neutral-300 dark:border-neutral-700 px-2 py-1 text-[10px] font-medium text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 disabled:opacity-60 cursor-pointer"
          title={
            resyncAllowed
              ? pendingScreenshotCount > 0
                ? "Retry task record and pending screenshot uploads"
                : synced
                ? "Resync this entry"
=======
          disabled={syncing || !resyncAllowed || requiresReview}
          className="inline-flex items-center gap-1 rounded-full border border-neutral-300 dark:border-neutral-700 px-2 py-1 text-[10px] font-medium text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 disabled:opacity-60 cursor-pointer"
          title={
            requiresReview
              ? "Review the matching Bubble session first"
              : resyncAllowed
              ? pendingScreenshotCount > 0
                ? "Retry task record and pending screenshot uploads"
              : synced
                ? "Resync this entry"
                : remoteMissing
                  ? "Create a replacement Bubble productivity record"
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
                : "Retry sync"
              : resyncBlockedReason
          }
        >
          <RotateCcw className={`w-3 h-3 ${syncing ? "animate-spin" : ""}`} />
<<<<<<< HEAD
          {syncing ? "Syncing..." : "Resync"}
=======
          {syncing ? "Syncing..." : requiresReview ? "Review required" : "Resync"}
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
        </button>
      </div>
    </div>
  );
}

export default ActivityLogs;
