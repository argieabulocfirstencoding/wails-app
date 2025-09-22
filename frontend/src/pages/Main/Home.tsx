import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import { Plus, Search, RefreshCcw } from "lucide-react";
import { useEffect } from "react";
import { CheckAuth } from "@/../wailsjs/go/app/App";

const Home = () => {
  return (
    <MainLayout>
      <div className="p-6 border-b-2 flex justify-between items-center">
        <h1 className="text-2xl font-extrabold">All Task</h1>
        <Plus className="cursor-pointer" />
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
            <RefreshCcw color="green" />
          </div>
        </div>
      </div>
      <div className="flex justify-between items-center px-6 font-bold pb-2 border-b mb-4">
        <span>Contract</span>
        <span>This week's hours</span>
      </div>
      <div className="">
        <TaskItem />
      </div>
    </MainLayout>
  );
};

const TaskItem = () => {
  const navigate = useNavigate();

  useEffect(() => {
    async function fetchUser() {
      const username = localStorage.getItem("user");
      const auth = await CheckAuth(username);

      if (!auth) {
        navigate("/login");
      }
    }

    fetchUser();
  }, []);

  return (
    <>
      <div
        className="grid grid-cols-2 px-6 cursor-pointer"
        onClick={() => navigate("/task")}
      >
        <div className="flex flex-col">
          <span className="text-green-500 line-clamp-2">
            Lorem ipsum dolor sit, amet consectetur adipisicing elit. Lorem
            ipsum dolor, sit amet consectetur adipisicing elit. Excepturi magni
            doloribus beatae harum ea labore optio nam corrupti provident. Id
            excepturi odio commodi recusandae neque corrupti distinctio
            accusantium ducimus consectetur.
          </span>
          <span className="truncate font-bold">
            uam qui odit similique aliquid ipsum labore earum officia.
          </span>
          <span className="text-xs font-semibold text-gray-500">
            Time last tracked Sep 12, 2025
          </span>
        </div>
        <div></div>
      </div>
      <hr className="mt-5" />
    </>
  );
};

export default Home;
