import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import { Link } from "react-router-dom";
import { intervalToDuration } from "date-fns";
import { Plus, Search, RefreshCcw } from "lucide-react";
import { Input } from "@/components/ui/input";

const Home = () => {
  const [timeIn, setTimeIn] = useState<number | null>(null);
  const [timeOut, setTimeOut] = useState<number | null>(null);
  const [elapsedTime, setElapsedTime] = useState<number>(0);

  useEffect(() => {
    if (!timeIn || timeOut) return;

    const interval = setInterval(() => {
      const elapsed = Math.floor((Date.now() - timeIn) / 1000);
      setElapsedTime(elapsed);
    }, 1000);

    return () => clearInterval(interval);
  }, [timeIn, timeOut]);

  function handleTimeIn() {
    const time = Date.now();
    setTimeIn(time);
    setElapsedTime(0);
  }

  function handleTimeOut() {
    setTimeOut(Date.now());
  }

  function getFormattedTime() {
    if (timeIn) {
      const duration = intervalToDuration({ start: timeIn, end: Date.now() });

      const hours = duration.hours ?? 0;
      const minutes = duration.minutes ?? 0;
      const secs = duration.seconds ?? 0;

      return `${hours}h:${String(minutes).padStart(2, "0")}m:${String(
        secs
      ).padStart(2, "0")}s`;
    } else {
      return `--:--:--`;
    }
  }

  return (
    <MainLayout>
      <div className="flex flex-col h-full w-screen">
        <div className="flex-1">
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
          <div className="grid grid-cols-2 px-6">
            <div className="flex flex-col">
              <span className="text-green-500">
                Lorem ipsum dolor sit, amet consectetur adipisicing elit.
              </span>
              <span>
                uam qui odit similique aliquid ipsum labore earum officia.
              </span>
            </div>
            <div></div>
          </div>
        </div>
        <div className="flex px-8 py-4 border-t slight-shadow w-full justify-between">
          <img
            src="https://www.shutterstock.com/image-vector/user-profile-icon-vector-avatar-600nw-2558760599.jpg"
            className="rounded-full border-2 border-green-500 select-none"
            style={{ height: 70, width: 70 }}
          />
          <div className="flex gap-5 items-center">
            <span className="text-xl font-semibold text-gray-600">
              {getFormattedTime()}
            </span>
            <button
              className={`px-4 py-2 rounded ${
                timeOut ? "bg-gray-500" : timeIn ? "bg-red-500" : "bg-green-500"
              }
              } text-white font-bold cursor-pointer`}
              onClick={!timeIn ? handleTimeIn : handleTimeOut}
              disabled={Boolean(timeOut)}
            >
              {timeOut ? "Recorded" : timeIn ? "Time out" : "Time in"}
            </button>
          </div>
        </div>
      </div>
    </MainLayout>
  );
};

export default Home;
