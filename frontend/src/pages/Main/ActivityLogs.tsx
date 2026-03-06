import { useEffect, useState } from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import {
  History,
  LogIn,
  LogOut,
  AlertTriangle,
  UserCircle2,
} from "lucide-react";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import { models } from "@/../wailsjs/go/models";
import { GetTotalClientRecords } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import { Calendar22 } from "@/components/ui/calendar22";
import { fetchTodayClientSummary, secondsToHHMM } from "@/lib/utils";

const ActivityLogs = () => {
  const [activities, setActivities] = useState<models.ActivityLogRecord[]>();
  const { summaries, setSummary } = useAuthStore();
  const [date, setDate] = useState<Date>(new Date());

  useEffect(() => {
    fetchTodayClientSummary(date);
  }, [date]);

  useEffect(() => {
    async function fetchTodayClientSummary() {
      try {
        const dateOnly = date.toLocaleDateString("en-CA", {
          timeZone: "Asia/Manila",
        });

        const results = await GetTotalClientRecords(dateOnly);
        console.log("results", results);
        setSummary(results);
      } catch (error) {
        console.log("Error in fetchActivity in activity logs ", error);
      }
    }

    fetchTodayClientSummary();
  }, [date]);

  

  console.log(secondsToHHMM(3661)); // "01:01"
  console.log(secondsToHHMM(5400)); // "01:30"
  console.log(secondsToHHMM(90000)); // "25:00"

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
        <div className="absolute top-3 z-999 right-4 flex justify-end items-end">
          <Calendar22 setDate={setDate} date={date} />
        </div>
        {summaries && summaries.length > 0 && (
          <section
            className="
            rounded-2xl
            border border-neutral-200 dark:border-neutral-800
            bg-white dark:bg-neutral-900
            overflow-hidden
          "
          >
            <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800 flex items-center justify-between">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
                Client
              </span>
              {/* <History className="w-4 h-4 text-neutral-400 dark:text-neutral-500" /> */}

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
            {/* <History className="w-4 h-4 text-neutral-400 dark:text-neutral-500" /> */}
          </div>

          {/* Body */}
          <div className="divide-y divide-neutral-200 dark:divide-neutral-800">
            {activities?.length === 0 ? (
              <div className="px-6 py-6 text-sm text-neutral-500 dark:text-neutral-400">
                No activity logs yet.
              </div>
            ) : (
              activities?.map((activity, idx) => (
                <ActivityBannerSet
                  activity={activity}
                  key={activity.attendance.ID + idx}
                />
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
      <AppFooterInfo className="mt-6" />
    </SettingsLayout>
  );

  function ActivityBannerSet({
    activity,
  }: {
    activity: models.ActivityLogRecord;
  }) {
    const { tasks } = useAuthStore();

    function getTasksFromId(id: string) {
      return tasks.find((task) => task.taskId === id) ?? null;
    }

    return (
      <>
        <ActivityBanner
          label="Logged In"
          description="FES App Tracker session started"
          iso_time={activity.attendance.TimeIn}
          synced_at={activity.attendance.SyncedAt}
        />

        {activity.records.map((record, idx) => {
          const task = getTasksFromId(record.TaskID);

          return (
            <ActivityBanner
              label="Task Recorded"
              description={`${
                task?.title?.slice(0, 50) +
                (task?.title && task.title.length > 50 ? "..." : "")
              } task has been recorded`}
              iso_time={record.StartedAt}
              synced_at={record.SyncedAt}
            />
          );
        })}

        {activity.attendance.TimeOut && (
          <ActivityBanner
            label="Logged Out"
            description="FES App Tracker session ended"
            iso_time={activity.attendance.TimeOut}
            synced_at={activity.attendance.SyncedAt}
          />
        )}
      </>
    );

    function ActivityBanner({
      label,
      description,
      iso_time,
      synced_at,
    }: {
      label: string;
      description: string;
      iso_time: string;
      synced_at: string;
    }) {
      return (
        <div
          key={label + iso_time}
          className="px-4 py-4 flex items-start justify-between gap-4"
        >
          <div className="flex items-start gap-3">
            {renderIcon(label)}
            <div className="space-y-0.5">
              <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                {label}
              </p>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                {description}
              </p>
            </div>
          </div>
          <div className="flex flex-col items-end gap-1">
            <span className="text-[11px] text-neutral-500 dark:text-neutral-400">
              {(() => {
                const t = formatTimestamp(iso_time);
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

            <div>
              <span className="text-[9px] text-emerald-400">Synced at:</span>
              <span className="text-[9px] text-neutral-500 dark:text-neutral-400">
                {(() => {
                  const t = formatTimestamp(synced_at);
                  return (
                    <div className="flex flex-col items-end leading-tight">
                      <span className="text-neutral-700 dark:text-neutral-300">
                        {t.date}
                      </span>
                      <span className="text-neutral-500 dark:text-neutral-400">
                        {t.time}
                      </span>
                    </div>
                  );
                })()}
              </span>
            </div>
          </div>
        </div>
      );
    }
  }
};

export default ActivityLogs;
