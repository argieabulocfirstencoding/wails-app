import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import {
  Search,
  RotateCcw, // Changed from RefreshCcw
  CheckCircle,
  TriangleAlert,
  ArrowDownUp,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  GetActiveTaskRecord,
  GetTasks,
  GetCurrentAttendance,
  GetLastTaskRecords,
  FetchActiveTask,
  FetchUserDetails,
} from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import Loader from "@/components/custom/Loader";
import { models, types } from "@/../wailsjs/go/models";
import { EventsOn } from "@/../wailsjs/runtime/runtime";
import { formatDistance } from "date-fns";
import { fetchTodayClientSummary } from "@/lib/utils";

interface TaskProps {
  taskId: string | null;
  title: string;
  client: string;
}

const Home = () => {
  const navigate = useNavigate();
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [taskSyncWarning, setTaskSyncWarning] = useState("");
  const [taskRecords, setTaskRecords] = useState<models.ExpTaskRecord[]>();
  const [nowTick, setNowTick] = useState<number>(Date.now());
  const {
    taskID,
    setFetchedActiveRecord,
    fetchedActiveRecord,
    setTaskID,
    online,
    tasks,
    setTasks,
    synced,
    attendanceSynced,
    setAttendanceSynced,
    initializedTask,
    setInitializedTask,
    summaries,
  } = useAuthStore();

  useEffect(() => {
    if (!EventsOn) return;

    async function fetchDetails() {
      try {
        const response: models.AuthUser = await FetchUserDetails();
        console.log("RESPONSE", response);

        useAuthStore.getState().setUser({
          uid: response.localId,
          ...response,
        });
      } catch (err) {
        console.error(err);
      }
    }

    const off = EventsOn("employee_details_fetched", () => {
      fetchDetails();
    });

    fetchDetails();

    return () => off();
  }, []);

  useEffect(() => {
    fetchTodayClientSummary();
  }, []);

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setNowTick(Date.now());
    }, 1000);

    return () => window.clearInterval(intervalId);
  }, []);

  useEffect(() => {
    refreshTaskRecords();
    refreshActiveRecord(true);
  }, []);

  useEffect(() => {
    if (!EventsOn) return;
    const off = EventsOn("record-synced", () => {
      refreshTaskRecords();
      refreshActiveRecord(true);
    });

    return () => off();
  }, []);

  useEffect(() => {
    if (!EventsOn) return;
    const off = EventsOn("browser-records-synced", async () => {
      await refreshTaskRecords();
      const fetchedTasks: any = await GetTasks(false);
      if (fetchedTasks) setTasks(fetchedTasks);
      await fetchTodayClientSummary();
    });

    return () => off();
  }, []);
  useEffect(() => {
    const off = EventsOn("attendance-synced", (synced: boolean) => {
      setAttendanceSynced(synced);
      fetchCurrentAttendance();
      refreshActiveRecord(true);
    });

    return () => off();
  }, []);

  useEffect(() => {
    if (!EventsOn) return;

    const off = EventsOn("tasks-sync-warning", (message: string) => {
      setTaskSyncWarning(
        typeof message === "string" ? message : String(message),
      );
    });

    return () => off();
  }, []);

  useEffect(() => {
    async function fetchUser() {
      const user = useAuthStore.getState().user;
      if (!user) {
        navigate("/login");
      }
    }

    fetchInitialTasks(); // initial load uses 'loading'
    fetchUser();
    // fetchActiveRecord moved into fetchInitialTasks
  }, []);

  // Initial page load (full page loader)
  async function fetchInitialTasks() {
    if (!online) return;
    setTaskSyncWarning("");

    // Initial task boolean helper that is stored in context so that it wont repeat on rerender
    if (!initializedTask) {
      setInitializedTask(true);
    }
    setLoading(true);
    try {
      if (typeof GetTasks !== "function") {
        console.warn(
          "GetTasks binding not available (wails dev server not running?)",
        );
      } else {
        // pass required boolean argument (e.g. includeArchived = true)
        const fetchedTasks: any = await GetTasks(false);
        if (fetchedTasks) setTasks(fetchedTasks);
      }
      await refreshActiveRecord(true);
    } catch (error) {
      console.log("Error fetching initial tasks:", error);
    } finally {
      setLoading(false);
    }
  }

  async function refreshTaskRecords() {
    const task_records = await GetLastTaskRecords();

    console.log("Task Records: ", task_records);
    setTaskRecords(task_records);
    await refreshActiveRecord(true);
  }

  async function refreshActiveRecord(force = false) {
    const { online: currentOnline, fetchedActiveRecord: currentFetchedActiveRecord } =
      useAuthStore.getState();

    if (!currentOnline) return;

    if (!force && currentFetchedActiveRecord) {
      return;
    }

    const taskID = await GetActiveTaskRecord();
    setTaskID(taskID || null);

    if (!force) {
      setFetchedActiveRecord(true);
    }
  }

  async function fetchCurrentAttendance() {
    await GetCurrentAttendance().then((attendance) => {
      if (attendance) {
        console.log(attendance);
        setAttendanceSynced(attendance.sync_status === "synced");
      }
    });
  }

  // This checks in each render to see if user current attendance session is fully synced
  // Upgrade: Need to check for an event and database changes especially on screenshot will turn this to unsynced
  useEffect(() => {
    async function fetchAttendanceSyncStatus() {
      await fetchCurrentAttendance();
    }

    fetchAttendanceSyncStatus();
  }, []);

  async function fetchFESActiveRecord() {
    try {
      await FetchActiveTask();
    } catch (e) {
      console.error(e);
    }
  }

  // Refresh (button)
  async function fetchTasks() {
    if (!online) return;
    setTaskSyncWarning("");
    setIsRefreshing(true);
    try {
      if (typeof GetTasks !== "function") {
        console.warn(
          "GetTasks binding not available (wails dev server not running?)",
        );
      } else {
        // pass required boolean argument (e.g. includeArchived = true)
        const fethedTasks: any = await GetTasks(true);
        if (fethedTasks) {
          console.log(fethedTasks);
          setTasks(fethedTasks);
        }
      }

      await refreshActiveRecord(true);
    } catch (error) {
      console.error("Error fetching tasks:", error);
    } finally {
      // keep spinner visible briefly for smoother UX
      setTimeout(() => {
        setIsRefreshing(false);
        console.log("refresh finished, isRefreshing set false");
      }, 600);
    }
  }

  const sortedTasks = useMemo(() => {
    if (!tasks) return []; // only check tasks

    const recordsByTask: Record<string, models.ExpTaskRecord[]> = {};

    // 1. Pre-process summaries to combine TotalDuration by TaskID
    const combinedSummaries = (summaries ?? []).reduce(
      (acc, curr) => {
        const id = curr.TaskID;
        if (!acc[id]) {
          // Create a copy so we don't mutate the original summary object
          acc[id] = { ...curr };
        } else {
          // If ID exists, just add the duration
          acc[id].TotalDuration += curr.TotalDuration;
          // Optional: If Padagan is true in any record, keep it true for the summary
          if (curr.Padagan) acc[id].Padagan = true;
        }
        return acc;
      },
      {} as Record<string, any>,
    );

    (taskRecords ?? []).forEach((record) => {
      if (!recordsByTask[record.task_id]) recordsByTask[record.task_id] = [];
      recordsByTask[record.task_id].push(record);
    });

    // Step 2: Determine the active task
    const activeTaskId =
      (taskRecords ?? []).find((r) => !r.ended_at)?.task_id ?? null;

    // Step 3: Map tasks with their records and compute latest ended_at timestamp
    let mappedTasks = tasks.map((task) => {
      const records = recordsByTask[task.taskId] ?? [];
      const latestEndedAt = records.length
        ? Math.max(
            ...records.map((r) =>
              r.ended_at ? new Date(r.ended_at).getTime() : 0,
            ),
          )
        : 0;

      const recordSummary = combinedSummaries[task.taskId] || null;
      const runningRecord = records.find((r) => !r.ended_at);
      const runningStartedAt = runningRecord?.started_at
        ? new Date(runningRecord.started_at).getTime()
        : NaN;
      const runningSeconds =
        runningRecord && Number.isFinite(runningStartedAt)
          ? Math.max(0, Math.floor((nowTick - runningStartedAt) / 1000))
          : 0;
      const displayDurationSeconds =
        (recordSummary?.TotalDuration ?? 0) + runningSeconds;

      return {
        ...task,
        records,
        active: task.taskId === activeTaskId,
        latestEndedAt,
        recordSummary,
        displayDurationSeconds,
      };
    });

    // Step 4: Filter tasks based on default "only tasks with records"
    const hasAnyRecords = mappedTasks.some((t) => t.records.length > 0);

    if (!searchQuery.trim()) {
      mappedTasks = hasAnyRecords
        ? mappedTasks.filter((t) => t.records.length > 0)
        : mappedTasks; // if no records exist, show all
    }

    // Step 5: Apply search filter if there is a query (search all tasks)
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      mappedTasks = mappedTasks.filter((t) =>
        `${t.title} - ${t.client}`.toLowerCase().includes(q),
      );
    }

    // Step 6: Sort tasks: active first, then by latest ended_at descending
    mappedTasks.sort((a, b) => {
      if (a.active) return -1;
      if (b.active) return 1;
      return b.latestEndedAt - a.latestEndedAt;
    });

    return mappedTasks;
  }, [tasks, taskRecords, taskID, searchQuery, summaries, nowTick]);

  return (
    <MainLayout>
      {taskSyncWarning && (
        <div className="mx-4 mt-4 rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900 shadow-sm dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100">
          Task sync warning: {taskSyncWarning}
        </div>
      )}
      {/* Full-screen refresh overlay – now covers the whole app */}
      {isRefreshing && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-white/70 dark:bg-black/60 backdrop-blur-sm" />
          <div className="relative z-10 flex flex-col items-center gap-3 p-4">
            <div className="w-14 h-14 rounded-full border-2 border-green-500 flex items-center justify-center bg-white dark:bg-neutral-900">
              <div
                className="animate-spin"
                style={{ animationDirection: "reverse" }}
              >
                <RotateCcw
                  size={28}
                  className="text-green-600"
                  stroke="currentColor"
                  strokeWidth={2.2}
                  fill="none"
                  style={{ animationDirection: "reverse" }}
                />
              </div>
            </div>
            <span className="text-sm font-medium text-green-700 dark:text-green-300">
              Refreshing tasks...
            </span>
          </div>
        </div>
      )}

      {/* Top sticky area: title, sync status, search, column headers */}
      <div className="sticky top-0 z-20 bg-background/70 backdrop-blur-md border-b border-gray-200 dark:border-gray-800">
        <div className="max-w-5xl mx-auto">
          {/* Header */}
          <div className="px-4 sm:px-6 pt-4 pb-3 flex items-center justify-between">
            <h1 className="text-xl sm:text-2xl font-bold text-foreground">
              All tasks
            </h1>

            {synced && (
              <div className="flex items-center gap-3 text-xs sm:text-sm">
                {attendanceSynced ? (
                  <div
                    className="inline-flex items-center gap-1 text-green-600 dark:text-green-400"
                    title="Attendance synced"
                  >
                    <span>Synced</span>
                    <CheckCircle className="w-4 h-4" />
                  </div>
                ) : (
                  <div className="flex items-center gap-1 text-foreground">
                    <span className="flex items-center gap-1 text-[11px] sm:text-xs select-none text-amber-500">
                      Not synced
                      <TriangleAlert size={12} />
                    </span>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Search + Refresh */}
          <div className="px-4 sm:px-6 pb-3 flex items-center gap-4">
            <div className="flex flex-1 items-center gap-3 rounded-md border border-gray-300 dark:border-gray-700 bg-white dark:bg-neutral-900 px-3 py-2">
              <Search className="w-4 h-4 text-gray-500 dark:text-gray-400" />
              <input
                type="text"
                placeholder="Search tasks..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full bg-transparent text-sm text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 outline-none border-none"
              />
            </div>

            {/* Refresh button with spinner */}
            {/* <button
              onClick={() => {
                console.log("Refresh button clicked");
                fetchFESActiveRecord();
              }}
              disabled={isRefreshing || loading}
              className={`
                flex items-center justify-center
                w-10 h-10 rounded-full border-2 border-green-500
                bg-transparent
                transition-all duration-200
                ${
                  isRefreshing || loading
                    ? "cursor-not-allowed opacity-60"
                    : "cursor-pointer hover:bg-transparent dark:hover:bg-transparent"
                }
              `}
              type="button"
              title="Refresh tasks"
              aria-label="Refresh tasks"
            >
              <div
                className={
                  isRefreshing
                    ? ""
                    : "transform transition-transform duration-500 ease hover:-scale-120"
                }
              >
                <ArrowDownUp
                  size={20}
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2.2}
                  className="text-green-600"
                  style={{ color: "#16a34a" }}
                />
              </div>
            </button> */}

            {/* Refresh button with spinner */}
            <button
              onClick={() => {
                console.log("Refresh button clicked");
                fetchTasks();
              }}
              disabled={isRefreshing || loading}
              className={`
                flex items-center justify-center
                w-10 h-10 rounded-full border-2 border-green-500
                bg-transparent
                transition-all duration-200
                ${
                  isRefreshing || loading
                    ? "cursor-not-allowed opacity-60"
                    : "cursor-pointer hover:bg-transparent dark:hover:bg-transparent"
                }
              `}
              type="button"
              title="Refresh tasks"
              aria-label="Refresh tasks"
            >
              <div
                className={
                  isRefreshing
                    ? "animate-spin transform"
                    : "transform transition-transform duration-500 ease-in-out hover:-rotate-180"
                }
                style={{ animationDirection: "reverse" }}
              >
                <RotateCcw
                  size={20}
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2.2}
                  className="text-green-600"
                  style={{ color: "#16a34a" }}
                />
              </div>
            </button>
          </div>

          {/* Task list column headers */}
          <div className="px-4 sm:px-6 pb-2 pt-1 flex justify-between items-center border-t border-gray-200 dark:border-gray-700">
            <span className="text-[11px] sm:text-xs font-semibold tracking-wide uppercase text-gray-600 dark:text-gray-400">
              Contract
            </span>
            <span className="text-[11px] sm:text-xs font-semibold tracking-wide uppercase text-gray-600 dark:text-gray-400">
              This day&apos;s hours
            </span>
          </div>
        </div>
      </div>

      {/* Task List */}
      <div className="max-w-5xl mx-auto">
        {loading || !tasks ? (
          <div className="w-full min-h-100 flex items-center justify-center flex-col py-10">
            <Loader />
            <span className="mt-4 text-sm text-gray-600 dark:text-gray-400">
              Fetching tasks...
            </span>
          </div>
        ) : sortedTasks.length > 0 ? (
          sortedTasks.map((task, index) => (
            <TaskItem
              key={task.taskId ?? index}
              task={task}
              active={task.active}
            />
          ))
        ) : (
          <div className="mt-6 text-center text-sm text-gray-500 dark:text-gray-400">
            No tasks found.
          </div>
        )}
      </div>
    </MainLayout>
  );
};

interface TaskItemProps {
  task: any;
  active: boolean;
}

const secondsToHHMMSS = (seconds: number): string => {
  const total = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(
      2,
      "0",
    )}`;
  }
  return `${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
};

const TaskItem = ({ task, active }: TaskItemProps) => {
  const navigate = useNavigate();

  return (
    <>
      <div
        className={`grid grid-cols-[minmax(0,1fr)_auto] gap-4 px-6 py-4 cursor-pointer transition-all duration-200
          ${
            active
              ? "bg-green-100 dark:bg-green-900/60 border-l-4 border-green-600 shadow-sm"
              : "hover:bg-green-50 dark:hover:bg-neutral-800 border-l-4 border-transparent hover:border-green-500"
          }`}
        onClick={() => navigate("/task/" + task.taskId)}
      >
        <div className="flex flex-col">
          <span
            className={`line-clamp-2 text-sm sm:text-base ${
              active
                ? "font-semibold text-gray-800 dark:text-white"
                : "font-semibold text-green-700 dark:text-green-400"
            }`}
          >
            {task.title}
          </span>
          <span
            className={`truncate text-xs sm:text-sm mt-0.5 ${
              active
                ? "font-medium text-gray-900 dark:text-gray-200"
                : "font-medium text-gray-700 dark:text-gray-300"
            }`}
          >
            {task.client}
          </span>
          <span
            className={`mt-1 text-[11px] sm:text-xs font-medium ${
              active
                ? "text-green-600 dark:text-green-400"
                : "text-gray-500 dark:text-gray-400"
            }`}
          >
            Time last tracked:{" "}
            {task.records.length
              ? formatDistance(
                  new Date(
                    task.records[0].ended_at || task.records[0].started_at,
                  ),
                  new Date(),
                  { addSuffix: true },
                )
              : "--"}
          </span>
        </div>

        {/* Right column placeholder for future hours */}
        <div className="flex items-start justify-end">
          <span className="text-xs sm:text-sm font-medium text-gray-700 dark:text-gray-300 opacity-70">
            {task.recordSummary || task.displayDurationSeconds > 0
              ? secondsToHHMMSS(task.displayDurationSeconds) + " hrs"
              : "--"}
          </span>
        </div>
      </div>
      <hr className="border-gray-200 dark:border-gray-700" />
    </>
  );
};

export default Home;
