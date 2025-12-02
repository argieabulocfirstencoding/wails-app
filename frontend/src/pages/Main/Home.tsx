import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import {
  Plus,
  Search,
  RefreshCcw,
  CheckCircle,
  TriangleAlert,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
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
  } = useAuthStore();
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    async function fetchUser() {
      const user = useAuthStore.getState().user;
      if (!user) {
        navigate("/login");
      }
    }

    if (!fetchedActiveRecord) fetchTasks();
    fetchUser();
  }, []);

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

  useEffect(() => {
    fetchCurrentAttendance();
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

  async function fetchTasks() {
    setLoading(true);
    setIsRefreshing(true);

    try {
      const fethedTasks = await GetTasks(online);
      if (fethedTasks) {
        console.log(fethedTasks);
        setTasks(fethedTasks);
      }

      await fetchActiveRecord();
    } catch (error) {
      console.log("Error fetching tasks:", error);
    } finally {
      setIsRefreshing(false);
      setLoading(false);
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

  const filteredTasks = tasks.filter(
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
                  onClick={() => SyncCurrentAttendance()}
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
          <div
            className="border-2 border-green-500 rounded-full p-2 cursor-pointer"
            onClick={fetchTasks}
          >
            <RefreshCcw
              color="green"
              className={isRefreshing ? "animate-spin" : ""}
            />
          </div>
        </div>

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
                key={task.taskId} // better to use unique ID instead of index
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
              : "hover:bg-gray-50 dark:hover:bg-gray-800"
          }`}
        onClick={() => navigate("/task/" + taskId)}
      >
        <div className="flex flex-col">
          <span
            className={`line-clamp-2 font-semibold ${
              active
                ? "text-green-700 dark:text-green-400"
                : "text-green-500 dark:text-green-300"
            }`}
          >
            {title}
          </span>
          <span
            className={`truncate ${
              active
                ? "font-extrabold text-gray-900 dark:text-white"
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
      <hr className="border-gray-200 dark:border-gray-700" />
    </>
  );
};

export default Home;
