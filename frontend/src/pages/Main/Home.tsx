import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import {
  Search,
  RotateCcw, // Changed from RefreshCcw
  CheckCircle,
  TriangleAlert,
} from "lucide-react";
import { useEffect, useState } from "react";
import {
  CreateTask,
  GetActiveTaskRecord,
  GetTasks,
  GetCurrentAttendance,
  GetLastTaskRecords,
  SyncUnsyncedAttendance,
} from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import { Button } from "@/components/ui/button";
import Loader from "@/components/custom/Loader";
import SpinnerLoader from "@/components/custom/SpinnerLoader";
import { models } from "wailsjs/go/models";

interface TaskProps {
  taskId: string | null;
  title: string;
  client: string;
}

const Home = () => {
  const navigate = useNavigate();
  const [newTask, setNewTask] = useState<TaskProps>({
    taskId: null,
    title: "",
    client: "",
  });
  const [open, setOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isSyncing, setSyncing] = useState(false);
  const [taskRecords, setTaskRecords] = useState<models.ExpTaskRecord[]>();
  const [showingTasks, setShowingTasks] = useState();
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
  } = useAuthStore();

  useEffect(() => {
    async function fetchAllTaskRecords() {
      const task_records = await GetLastTaskRecords();

      console.log("Task Records: ", task_records);
      setTaskRecords(task_records);
    }

    fetchAllTaskRecords();
  }, []);

  useEffect(() => {
    if (searchQuery.length > 0) {
    }
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
    if (!online || initializedTask) return;

    // Initial task boolean helper that is stored in context so that it wont repeat on rerender
    setInitializedTask(true);
    setLoading(true);
    try {
      if (typeof GetTasks !== "function") {
        console.warn(
          "GetTasks binding not available (wails dev server not running?)"
        );
      } else {
        // pass required boolean argument (e.g. includeArchived = true)
        const fetchedTasks: any = await GetTasks(true);
        if (fetchedTasks) setTasks(fetchedTasks);
      }
      await fetchActiveRecord();
    } catch (error) {
      console.log("Error fetching initial tasks:", error);
    } finally {
      setLoading(false);
    }
  }

  async function fetchActiveRecord() {
    if (online && !fetchedActiveRecord) {
      const taskID = await GetActiveTaskRecord();
      if (taskID) {
        setTaskID(taskID);
      }
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
      fetchCurrentAttendance();
    }

    fetchAttendanceSyncStatus();
  }, []);

  // Not used for now
  async function createTask() {
    if (!newTask?.title || !newTask?.client) return;

    try {
      const task = await CreateTask(newTask.title, newTask.client);
      if (task) {
        setNewTask({ taskId: null, title: "", client: "" });
        setOpen(false);
        fetchTasks();
      }
    } catch (error) {
      console.log("Error creating task: ", error);
    }
  }

  // Refresh (button)
  async function fetchTasks() {
    if (!online) return;
    setIsRefreshing(true);
    try {
      if (typeof GetTasks !== "function") {
        console.warn(
          "GetTasks binding not available (wails dev server not running?)"
        );
      } else {
        // pass required boolean argument (e.g. includeArchived = true)
        const fethedTasks: any = await GetTasks(true);
        if (fethedTasks) {
          console.log(fethedTasks);
          setTasks(fethedTasks);
        }
      }

      await fetchActiveRecord();
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

  async function SyncCurrentAttendance() {
    setSyncing(true);
    await SyncUnsyncedAttendance();
    await fetchCurrentAttendance();

    setTimeout(() => {
      setSyncing(false);
    }, 2000);
  }

  // TODO add sort so that previous tasks that have task records will be ordered by their task record
  const filteredTasks = (tasks ?? []).filter((task) => {
    if (!searchQuery) {
      if (taskRecords && taskRecords.length > 0) {
        // Only show tasks that have a corresponding record
        return taskRecords.some((record) => record.task_id === task.taskId);
      }
      // No taskRecords, show all tasks
      return true;
    }

    // If searching, filter by searchQuery
    const lowerQuery = searchQuery.toLowerCase();
    return (
      task.title.toLowerCase().includes(lowerQuery) ||
      task.client.toLowerCase().includes(lowerQuery)
    );
  });

  return (
    <MainLayout>
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
                {isSyncing ? (
                  <div className="flex items-center gap-1 text-foreground">
                    <span>Syncing</span>
                    <SpinnerLoader className="scale-75" />
                  </div>
                ) : attendanceSynced ? (
                  <div className="flex items-center gap-1 text-green-600 dark:text-green-400 select-none">
                    <span>Synced</span>
                    <CheckCircle className="w-4 h-4" />
                  </div>
                ) : (
                  <div className="flex flex-col items-end gap-1 text-amber-500">
                    <span className="flex items-center gap-1 text-[11px] sm:text-xs select-none">
                      Not synced
                      <TriangleAlert size={12} />
                    </span>
                    <Button
                      className="h-8 px-3 bg-green-500 hover:bg-green-600 cursor-pointer text-xs sm:text-sm text-white"
                      onClick={SyncCurrentAttendance}
                    >
                      {isSyncing ? "Syncing..." : "Sync now"}
                    </Button>
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
                    : "cursor-pointer hover:bg-green-50 dark:hover:bg-green-900/20"
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
              This week&apos;s hours
            </span>
          </div>
        </div>
      </div>

      {/* Task List */}
      <div className="max-w-5xl mx-auto">
        {loading ? (
          <div className="w-full min-h-100 flex items-center justify-center flex-col py-10">
            <Loader />
            <span className="mt-4 text-sm text-gray-600 dark:text-gray-400">
              Fetching tasks...
            </span>
          </div>
        ) : filteredTasks && filteredTasks.length > 0 ? (
          filteredTasks
            .slice()
            .sort((a, b) =>
              a.taskId === taskID ? -1 : b.taskId === taskID ? 1 : 0
            )
            .map((task, index) => (
              <TaskItem
                key={task.taskId ?? index}
                taskId={task.taskId}
                title={task.title}
                client={task.client}
                active={task.taskId === taskID}
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
  taskId: string | null;
  title: string;
  client: string;
  active: boolean;
}

const TaskItem = ({ taskId, title, client, active }: TaskItemProps) => {
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
        onClick={() => navigate("/task/" + taskId)}
      >
        <div className="flex flex-col">
          <span
            className={`line-clamp-2 text-sm sm:text-base ${
              active
                ? "font-semibold text-gray-800 dark:text-white"
                : "font-semibold text-green-700 dark:text-green-400"
            }`}
          >
            {title}
          </span>
          <span
            className={`truncate text-xs sm:text-sm mt-0.5 ${
              active
                ? "font-medium text-gray-900 dark:text-gray-200"
                : "font-medium text-gray-700 dark:text-gray-300"
            }`}
          >
            {client}
          </span>
          <span
            className={`mt-1 text-[11px] sm:text-xs font-medium ${
              active
                ? "text-green-600 dark:text-green-400"
                : "text-gray-500 dark:text-gray-400"
            }`}
          >
            Time last tracked: --
          </span>
        </div>

        {/* Right column placeholder for future hours */}
        <div className="flex items-start justify-end">
          <span className="text-xs sm:text-sm font-medium text-gray-700 dark:text-gray-300 opacity-70">
            --
          </span>
        </div>
      </div>
      <hr className="border-gray-200 dark:border-gray-700" />
    </>
  );
};

export default Home;
