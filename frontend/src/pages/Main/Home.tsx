import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import {
  Plus,
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
  SyncUnsyncedAttendance,
} from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import Loader from "@/components/custom/Loader";
import SpinnerLoader from "@/components/custom/SpinnerLoader";

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

  const filteredTasks = (tasks ?? []).filter(
    (task) =>
      task.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      task.client.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <MainLayout>
      <div className="sticky top-0 bg-white dark:bg-neutral-900">
        {/* Header */}
        <div className="p-6 border-b-2 flex justify-between items-center dark:border-gray-700">
          <h1 className="text-2xl font-extrabold text-gray-900 dark:text-white">
            All Tasks
          </h1>
          {synced &&
            (isSyncing ? (
              <div className="flex gap-1 items-center">
                <span>Syncing</span>
                <SpinnerLoader />
              </div>
            ) : attendanceSynced ? (
              <div className="flex gap-1 text-green-500 select-none">
                <span>Synced</span>
                <CheckCircle />
              </div>
            ) : (
              <div className="flex flex-col gap-2 text-amber-500">
                <span className="flex items-center gap-1 text-xs select-none">
                  Not Synced <TriangleAlert size={12} />
                </span>
                <Button
                  className="bg-green-500 hover:bg-green-700 cursor-pointer text-white"
                  onClick={SyncCurrentAttendance}
                >
                  Sync Now
                </Button>
              </div>
            ))}
        </div>

        {/* Search + Refresh */}
        <div className="flex items-center p-5 gap-5">
          <div className="p-2 border flex gap-4 rounded-md flex-grow dark:border-gray-700">
            <Search className="text-gray-600 dark:text-gray-300" />
            <input
              type="text"
              placeholder="Search tasks..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="outline-none border-none w-full bg-transparent text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400"
            />
          </div>

          {/* Refresh button with spinner */}
          <button
            onClick={() => {
              console.log("Refresh button clicked");
              fetchTasks();
            }}
            disabled={isRefreshing || loading}
            className={`flex items-center justify-center w-10 h-10 rounded-full border-2 border-green-500 transition-all duration-200
              ${
                isRefreshing
                  ? "bg-transparent"
                  : "bg-transparent hover:bg-transparent"
              }
              ${
                isRefreshing || loading
                  ? "cursor-not-allowed opacity-60"
                  : "cursor-pointer"
              }`}
            type="button"
            title="Refresh tasks"
            aria-label="Refresh tasks"
          >
            <div
              className={
                isRefreshing
                  ? "animate-spin transform"
                  : "transform transition-transform duration-500 ease-in-out hover:rotate-180"
              }
            >
              <RotateCcw
                size={20}
                fill="none"
                stroke="currentColor" // ensure SVG stroke follows currentColor
                strokeWidth={2.2}
                className="text-green-600" // Tailwind color applied as currentColor
                style={{ color: "#16a34a" }} // inline fallback to override conflicting CSS
              />
            </div>
          </button>
        </div>

        {/* Full-screen refresh overlay */}
        {isRefreshing && (
          <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div className="absolute inset-0 bg-white/70 dark:bg-black/60 backdrop-blur-sm" />
            <div className="relative z-10 flex flex-col items-center gap-3 p-4">
              <div className="w-14 h-14 rounded-full border-2 border-green-500 flex items-center justify-center bg-white dark:bg-neutral-900">
                <div className="animate-spin">
                  <RotateCcw
                    size={28}
                    className="text-green-600"
                    stroke="currentColor"
                    strokeWidth={2.2}
                    fill="none"
                  />
                </div>
              </div>
              <span className="text-green-700 dark:text-green-300 font-medium">
                Refreshing tasks...
              </span>
            </div>
          </div>
        )}

        {/* Task list header */}
        <div className="flex justify-between items-center px-6 font-bold pb-2 border-b dark:border-gray-700">
          <span className="text-gray-800 dark:text-gray-200">Contract</span>
          <span className="text-gray-800 dark:text-gray-200">
            This week's hours
          </span>
        </div>
      </div>

      {/* Task List */}
      <div className="">
        {loading ? (
          <div className="w-full min-h-100 flex items-center justify-center flex-col">
            <Loader />
            <span className="mt-5">Fetching Tasks</span>
          </div>
        ) : filteredTasks && filteredTasks.length > 0 ? (
          // Sort so active task comes first
          filteredTasks
            .slice() // copy to avoid mutating original array
            .sort((a, b) =>
              b.taskId === taskID ? 1 : a.taskId === taskID ? -1 : 0
            )
            .map((task, index) => (
              <TaskItem
                key={task.taskId ?? index} // fallback to index if id is null
                taskId={task.taskId}
                title={task.title}
                client={task.client}
                active={task.taskId === taskID}
              />
            ))
        ) : (
          <div className="mt-5 text-center text-gray-500 dark:text-gray-400">
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
        className={`grid grid-cols-[1fr_50px] px-6 py-4 cursor-pointer transition-all duration-300 
          ${
            active
              ? "bg-green-100 dark:bg-green-900 border-l-4 border-green-600 shadow-sm"
              : "hover:bg-green-50 dark:hover:bg-gray-800 border-l-4 border-transparent hover:border-green-500"
          }`}
        onClick={() => navigate("/task/" + taskId)}
      >
        <div className="flex flex-col">
          <span
            className={`line-clamp-2 font-semibold ${
              active
                ? "font-extrabold text-gray-700 dark:text-white"
                : "font-bold text-green-700 dark:text-green-500"
            }`}
          >
            {title}
          </span>
          <span
            className={`truncate ${
              active
                ? "font-extrabold text-gray-900 dark:text-gray-300"
                : "font-bold text-gray-700 dark:text-gray-300"
            }`}
          >
            {client}
          </span>
          <span
            className={`text-xs font-medium ${
              active
                ? "text-green-600 dark:text-green-400"
                : "text-gray-500 dark:text-gray-400"
            }`}
          >
            Time last tracked: --
          </span>
        </div>
      </div>
      <hr className="border-gray-300 dark:border-gray-700" />
    </>
  );
};

export default Home;
