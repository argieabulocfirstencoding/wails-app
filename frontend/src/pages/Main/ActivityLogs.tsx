import React from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import { History, LogIn, LogOut, AlertTriangle } from "lucide-react";

type ActivityLog = {
  event: string;
  detail?: string;
  timestamp: string; // ISO or formatted
};

// Static sample data for demonstration purposes
const staticLogs: ActivityLog[] = [
  {
    event: "Logged in",
    detail: "FES App Tracker session started",
    timestamp: "2025-10-22T14:03:00Z",
  },
  {
    event: "Task synced successfully",
    detail: "5 unsynced task records pushed to cloud",
    timestamp: "2025-10-22T13:22:00Z",
  },
  {
    event: "Screenshot captured",
    detail: "Screen 1 • Auto-capture",
    timestamp: "2025-10-22T12:45:10Z",
  },
  {
    event: "Logged out",
    detail: "FES App Tracker session ended",
    timestamp: "2025-10-22T12:15:00Z",
  },
  {
    event: "Logged in",
    detail: "FES App Tracker session started",
    timestamp: "2025-10-21T09:45:00Z",
  },
  {
    event: "Attendance started",
    detail: "Workday session opened",
    timestamp: "2025-10-21T09:46:20Z",
  },
  {
    event: "Screenshot captured",
    detail: "Screen 2 • Auto-capture",
    timestamp: "2025-10-21T08:30:10Z",
  },
  {
    event: "Logged out",
    detail: "FES App Tracker session ended",
    timestamp: "2025-10-21T07:58:00Z",
  },
  {
    event: "Logged in",
    detail: "FES App Tracker session started",
    timestamp: "2025-10-20T16:10:00Z",
  },
  {
    event: "Logged out",
    detail: "FES App Tracker session ended",
    timestamp: "2025-10-20T13:40:00Z",
  },
];

const ActivityLogs = () => {
  const logs = staticLogs;

  const renderIcon = (event: string) => {
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

  const formatTimestamp = (iso: string) => {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return { date: iso, time: "" };

    const date = d.toLocaleDateString(undefined, {
      month: "2-digit",
      day: "2-digit",
      year: "numeric",
    });

    const time = d.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      hour12: true,
    });

    return { date, time };
  };

  return (
    <SettingsLayout>
      <div className="max-w-md mx-auto space-y-6">
        {/* Page title (match Notifications) */}
        <header className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
            Activity Logs
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
            Here you can manage your activity logs.
          </p>
        </header>

        {/* Main card */}
        <section
          className="
            rounded-2xl
            border border-neutral-200 dark:border-neutral-800
            bg-white dark:bg-neutral-900
            overflow-hidden
          "
        >
          {/* Label row */}
          <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800 flex items-center justify-between">
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
              Recent Activity
            </span>
            <History className="w-4 h-4 text-neutral-400 dark:text-neutral-500" />
          </div>

          {/* Body */}
          <div className="divide-y divide-neutral-200 dark:divide-neutral-800">
            {logs.length === 0 ? (
              <div className="px-6 py-6 text-sm text-neutral-500 dark:text-neutral-400">
                No activity logs yet.
              </div>
            ) : (
              logs.map((log, idx) => (
                <div
                  key={log.event + log.timestamp + idx}
                  className="px-4 py-4 flex items-start justify-between gap-4"
                >
                  <div className="flex items-start gap-3">
                    {renderIcon(log.event)}
                    <div className="space-y-0.5">
                      <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                        {log.event}
                      </p>
                      {log.detail && (
                        <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                          {log.detail}
                        </p>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <span className="text-[11px] text-neutral-500 dark:text-neutral-400">
                      {(() => {
                        const t = formatTimestamp(log.timestamp);
                        return (
                          <div className="flex flex-col items-end leading-tight">
                            <span className="text-[11px] text-neutral-700 dark:text-neutral-300">
                              {t.date}
                            </span>
                            <span className="text-[11px] text-neutral-500 dark:text-neutral-400">
                              {t.time}
                            </span>
                          </div>
                        );
                      })()}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </section>

        {/* Footnote */}
        <footer className="flex items-start gap-2 text-xs text-neutral-500 dark:text-neutral-400">
          <AlertTriangle className="w-3.5 h-3.5 mt-0.5 text-amber-500" />
          <p>
            Activity is based on your attendance sessions. Login and logout
            entries are created whenever you start or stop tracking.
          </p>
        </footer>
      </div>
    </SettingsLayout>
  );
};

export default ActivityLogs;
