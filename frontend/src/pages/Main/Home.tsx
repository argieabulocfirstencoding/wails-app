import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import { Plus, Search, RefreshCcw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  CreateTask,
  GetActiveTaskRecord,
  GetTasks,
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
  const {
    taskID,
    setFetchedActiveRecord,
    fetchedActiveRecord,
    setTaskID,
    online,
    tasks,
    setTasks,
    synced,
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
    if (!online) return;

    setLoading(true);
    setIsRefreshing(true);

    try {
      const fethedTasks = await GetTasks();
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

          {/* Add Task Dialog */}
          <Dialog open={open} onOpenChange={setOpen}>
            <form>
              <DialogTrigger asChild>
                <Plus className="cursor-pointer text-gray-700 dark:text-gray-300" />
              </DialogTrigger>
              <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                  <DialogTitle className="text-gray-900 dark:text-white">
                    Add Task
                  </DialogTitle>
                  <DialogDescription className="text-gray-600 dark:text-gray-400">
                    Make sure tasks are approved by your direct superior.
                  </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4">
                  <div className="grid gap-3">
                    <Label
                      htmlFor="task-title"
                      className="text-gray-700 dark:text-gray-300"
                    >
                      Title
                    </Label>
                    <Input
                      id="task-title"
                      placeholder="Ex. Training"
                      value={newTask.title}
                      onChange={(e) =>
                        setNewTask((prev) => ({
                          ...prev,
                          title: e.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="grid gap-3">
                    <Label
                      htmlFor="task-client"
                      className="text-gray-700 dark:text-gray-300"
                    >
                      Client Name
                    </Label>
                    <Input
                      id="task-client"
                      placeholder="Ex. Edward Collins"
                      value={newTask.client}
                      onChange={(e) =>
                        setNewTask((prev) => ({
                          ...prev,
                          client: e.target.value,
                        }))
                      }
                    />
                  </div>
                </div>
                <DialogFooter>
                  <DialogClose asChild>
                    <Button variant="outline">Cancel</Button>
                  </DialogClose>
                  <Button
                    type="submit"
                    onClick={createTask}
                    className="bg-green-600 hover:bg-green-700 cursor-pointer text-white"
                  >
                    Add
                  </Button>
                </DialogFooter>
              </DialogContent>
            </form>
          </Dialog>
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
        className={`grid grid-cols-[1fr_100px] px-6 py-4 cursor-pointer transition-all duration-300 
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
            Time last tracked Sep 12, 2025
          </span>
        </div>
      </div>
      <hr className="border-gray-200 dark:border-gray-700" />
    </>
  );
};

export default Home;
