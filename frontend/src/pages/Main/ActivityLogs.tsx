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
  SyncAttendanceByID,
  SyncUnsyncedTaskRecord,
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

const MANILA_TIMEZONE = "Asia/Manila";

const toDateOnly = (input: Date): string => {
  const year = input.getFullYear();
  const month = String(input.getMonth() + 1).padStart(2, "0");
  const day = String(input.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
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

  const date = parsed.toLocaleDateString("en-US", {
    timeZone: MANILA_TIMEZONE,
    month: "2-digit",
    day: "2-digit",
    year: "numeric",
  });

  const time = parsed.toLocaleTimeString("en-US", {
    timeZone: MANILA_TIMEZONE,
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

const ActivityLogs = () => {
  const [activities, setActivities] = useState<models.ActivityLogRecord[]>([]);
  const [date, setDate] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);
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
      try {
        const dateOnly = toDateOnly(targetDate);
        const [fetchedActivities, summaryResults] = await Promise.all([
          GetActivityLogRecord(dateOnly),
          GetTotalClientRecords(dateOnly),
        ]);
        setActivities(fetchedActivities ?? []);
        setSummary(summaryResults ?? []);
      } catch (error) {
        console.log("Error loading activity logs:", error);
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

  const clearSyncError = (key: string) => {
    setSyncErrorByKey((prev) => ({ ...prev, [key]: "" }));
  };

  const setSyncError = (key: string, message: string) => {
    setSyncErrorByKey((prev) => ({ ...prev, [key]: message }));
  };

  const syncTaskRecord = async (recordID: number, key: string) => {
    if (!online) {
      setSyncError(key, "Cannot sync while offline.");
      return;
    }

    setSyncing(key, true);
    clearSyncError(key);
    try {
      await SyncUnsyncedTaskRecord(recordID);
      await loadActivityData(date);
    } catch (error) {
      console.error("Failed to sync task record:", error);
      setSyncError(key, "Sync failed. Please retry.");
    } finally {
      setSyncing(key, false);
    }
  };

  const syncAttendanceRecord = async (attendanceID: number, key: string) => {
    if (!online) {
      setSyncError(key, "Cannot sync while offline.");
      return;
    }

    setSyncing(key, true);
    clearSyncError(key);
    try {
      await SyncAttendanceByID(attendanceID);
      await loadActivityData(date);
    } catch (error) {
      console.error("Failed to sync attendance:", error);
      setSyncError(key, "Sync failed. Please retry.");
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
      <div className="max-w-md mx-auto space-y-6">
        <header className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
            Activity Logs
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
            Here you can manage your activity logs.
          </p>
        </header>

        <div className="absolute top-3 z-999 right-4 flex justify-end items-end">
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
                          syncError={syncErrorByKey[key]}
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

      <Dialog open={syncConfirmOpen} onOpenChange={setSyncConfirmOpen}>
        <DialogContent className="sm:max-w-[420px] rounded-2xl bg-white dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-800 shadow-xl">
          <DialogHeader className="space-y-2">
            <DialogTitle className="text-lg sm:text-xl font-semibold text-neutral-900 dark:text-neutral-50">
              Confirm Resync
            </DialogTitle>
            <DialogDescription className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
              Resyncing this activity may override what was recorded by another
              tracker for the same session.
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
              Continue Resync
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
  onSync,
}: ActivityBannerProps) {
  const eventTime = formatTimestamp(isoTime);
  const syncedTime = formatTimestamp(syncedAt);
  const synced = isSynced(syncStatus, syncedAt);
  const currentStatus = statusLabel(syncStatus, syncedAt);

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
          {syncError && (
            <p className="text-[10px] text-rose-500 dark:text-rose-400">
              {syncError}
            </p>
          )}
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
            synced
              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
              : "bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
          }`}
        >
          {synced ? (
            <CheckCircle2 className="w-3 h-3" />
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
          disabled={syncing}
          className="inline-flex items-center gap-1 rounded-full border border-neutral-300 dark:border-neutral-700 px-2 py-1 text-[10px] font-medium text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 disabled:opacity-60 cursor-pointer"
          title={synced ? "Resync this entry" : "Retry sync"}
        >
          <RotateCcw className={`w-3 h-3 ${syncing ? "animate-spin" : ""}`} />
          {syncing ? "Syncing..." : "Resync"}
        </button>
      </div>
    </div>
  );
}

export default ActivityLogs;
