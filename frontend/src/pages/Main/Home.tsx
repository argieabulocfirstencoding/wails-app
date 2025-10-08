import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import { Plus, Search, RefreshCcw } from "lucide-react";
import { useEffect, useState } from "react";
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
  const [tasks, setTasks] = useState<TaskProps[]>([]);
  const {
    taskID,
    setFetchedActiveRecord,
    fetchedActiveRecord,
    setTaskID,
    online,
  } = useAuthStore();
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchUser() {
      // const username = localStorage.getItem("user");
      // const auth = await CheckAuth(username);

      const user = useAuthStore.getState().user;
      if (!user) {
        navigate("/login");
      }
    }

    fetchUser();
    fetchTasks();
    fetchActiveRecord();
    setLoading(false);
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
    if (!newTask?.title || !newTask?.client) {
      return;
    }
    try {
      const task = await CreateTask(newTask.title, newTask.client);
      if (task)
        setNewTask({
          taskId: null,
          title: "",
          client: "",
        });
      setOpen(false);

      fetchTasks();
    } catch (error) {
      console.log("Error creating task" + error);
    }
  }

  async function fetchTasks() {
    setLoading(true);
    try {
      const tasks = await GetTasks();
      if (tasks) {
        console.log(tasks);
        setTasks(tasks);
      }

      fetchActiveRecord();
    } catch (error) {
      console.log("error fetching tasks" + error);
    } finally {
      setTimeout(() => {
        setLoading(false);
      }, 1500);
    }
  }

  return (
    <MainLayout>
      <div className="p-6 border-b-2 flex justify-between items-center">
        <h1 className="text-2xl font-extrabold">All Task</h1>
        {/* ==================================================================== */}
        <Dialog open={open} onOpenChange={setOpen}>
          <form>
            <DialogTrigger asChild>
              <Plus className="cursor-pointer" />
            </DialogTrigger>
            <DialogContent className="sm:max-w-[425px]">
              <DialogHeader>
                <DialogTitle>Add Task</DialogTitle>
                <DialogDescription>
                  Make sure task made are approved by your direct superior.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4">
                <div className="grid gap-3">
                  <Label htmlFor="name-1">Title</Label>
                  <Input
                    id="name-1"
                    name="name"
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
                  <Label htmlFor="username-1">Client name</Label>
                  <Input
                    id="username-1"
                    name="username"
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
                  className="bg-green-500"
                >
                  Add
                </Button>
              </DialogFooter>
            </DialogContent>
          </form>
        </Dialog>
        {/* ==================================================================== */}
      </div>

      <div className="flex items-center p-5 gap-5">
        <div className="p-2 border flex gap-4 rounded-md flex-grow">
          <Search />
          <input
            type="text"
            name=""
            id=""
            className="outline-none border-none"
          />
        </div>
        <div className="flex gap-3">
          <div className="border-2 border-green-500 rounded-full p-2">
            <RefreshCcw color="green" onClick={fetchTasks} />
          </div>
        </div>
      </div>
      <div className="flex justify-between items-center px-6 font-bold pb-2 border-b">
        <span>Contract</span>
        <span>This week's hours</span>
      </div>
      <div className="flex flex-col min-h-100">
        {loading ? (
          <Loader />
        ) : tasks && tasks.length > 0 ? (
          tasks.map((task, index) => (
            <TaskItem
              key={index}
              taskId={task.taskId}
              title={task.title}
              client={task.client}
              active={task.taskId === taskID}
            />
          ))
        ) : (
          <div className="mt-5 text-center">No tasks yet.</div>
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
        className={`grid grid-cols-2 px-6 cursor-pointer py-5 ${
          active && "bg-green-200"
        }`}
        onClick={() => navigate("/task/" + taskId)}
      >
        <div className="flex flex-col">
          <span className="text-green-500 line-clamp-2">{title}</span>
          <span className="truncate font-bold">{client}</span>
          <span className="text-xs font-semibold text-gray-500">
            Time last tracked Sep 12, 2025
          </span>
        </div>
        <div></div>
      </div>
      <hr />
    </>
  );
};

export default Home;
