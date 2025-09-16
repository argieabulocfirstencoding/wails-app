import MainLayout from "@/layouts/MainLayout";
import { ChevronLeft, CirclePlay, StopCircle } from "lucide-react";
import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { intervalToDuration } from "date-fns";
import { TakeScreenshot, GetMonitorCount } from "@/../wailsjs/go/app/App";

const Task = () => {
  const navigate = useNavigate();
  const [isStarted, setStart] = useState(false);
  const [timeStarted, setStartTime] = useState<number>(0);
  const [timeEnded, setEndTime] = useState<number>();
  const [elapsedTime, setElapsedTime] = useState<number>(0);
  const [monitor, setMonitor] = useState<number>(0);
  const [screenshots, setScreenshots] = useState<string[]>([]);

  function startTime() {
    resetTimer();

    const timeStart = Date.now();
    setStart(true);
    localStorage.setItem("started", JSON.stringify(timeStart));
    setStartTime(timeStart);
  }

  function resetTimer() {
    setStartTime(0);
    setEndTime(0);
    setElapsedTime(0);
    localStorage.removeItem("ended");
    localStorage.removeItem("started");
  }

  function stopTime() {
    const timeStop = Date.now();
    setStart(false);
    setEndTime(timeStop);
    localStorage.setItem("ended", JSON.stringify(timeStop));
  }

  function getFormattedTime() {
    if (timeStarted) {
      const duration = intervalToDuration({
        start: timeStarted,
        end: timeEnded ? timeEnded : Date.now(),
      });

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

  async function screenshotNow() {
    const pictures = await TakeScreenshot();

    if (pictures) setScreenshots(pictures);
  }

  async function countMonitor() {
    const monitorCount = await GetMonitorCount();
    setMonitor(monitorCount);
  }

  function openPreview(base64: string) {
    const newWindow = window.open();
    if (newWindow) {
      newWindow.document.write(
        `<img src="${base64}" style="max-width:100%; height:auto;" />`
      );
    }
  }

  useEffect(() => {
    const timeStartRecord = localStorage.getItem("started");
    const timeStopRecord = localStorage.getItem("ended");

    if (timeStartRecord) setStartTime(JSON.parse(timeStartRecord));
    if (timeStopRecord) setEndTime(JSON.parse(timeStopRecord));

    countMonitor();
  }, []);

  useEffect(() => {
    let endTime = localStorage.getItem("ended");

    if (!isStarted || !timeStarted || endTime) return;

    const interval = setInterval(() => {
      const elapsed = Math.floor((Date.now() - timeStarted) / 1000);
      setElapsedTime(elapsed);
    }, 1000);

    return () => clearInterval(interval);
  }, [isStarted]);

  useEffect(() => {
    const timeStartRecord = localStorage.getItem("started");
    const timeStopRecord = localStorage.getItem("ended");

    if (timeStartRecord && !timeStopRecord) {
      setStartTime(JSON.parse(timeStartRecord));
      setStart(true);
    }
  }, []);

  return (
    <MainLayout>
      <div className="p-4">
        <button
          onClick={() => navigate("/")}
          className="flex gap-2 font-bold cursor-pointer"
        >
          <ChevronLeft />
          Back
        </button>
        <h1 className="text-2xl font-bold mt-2 line-clamp-2">
          WordPress Website Maker Needed for Custom Site Developer
        </h1>
        <div className="flex flex-col mt-1">
          <span className="font-extrabold text-gray-600">Omar Taha</span>
          <span className="text-[10px] text-gray-500 mt-[-6px]">Client </span>
        </div>
      </div>
      <hr />
      <div className="p-5">
        <div className="rounded border p-4">
          <span className="font-bold">Current session</span>
          <div className="flex justify-between items-center">
            <div className="flex flex-col">
              <span className="font-bold text-2xl">{getFormattedTime()}</span>
              <span className="text-sm">
                Time tracking {isStarted ? "on" : "off"}
              </span>
            </div>
            {isStarted ? (
              <StopButton onClick={() => stopTime()} />
            ) : (
              <StartButton onClick={() => startTime()} />
            )}
          </div>
          <hr className="my-2" />
          <div className="flex justify-between">
            <span className="font-semibold">What are you working on?</span>
            <span className="text-green-500 font-bold">Edit</span>
          </div>
          <div className="flex flex-col">
            <span className="text-sm">Memo:</span>
            <span className="text-sm">Activity tag:</span>
          </div>
        </div>

        <div className="flex flex-col mt-3 mb-1">
          <div className="flex justify-between">
            <h2 className="font-bold">Latest screenshot</h2>
            <button onClick={screenshotNow}>Screenshot</button>
          </div>
          <div className="flex justify-between">
            <span>Active monitors: {monitor}</span>
            <span>24 mins ago</span>
          </div>
        </div>
        <div>
          {screenshots.map((value, index) => (
            <img
              key={index}
              src={value}
              alt=""
              className="w-full border rounded-2xl mb-2 object-cover"
              onClick={() => openPreview(value)}
            />
          ))}
        </div>
      </div>
    </MainLayout>
  );
};

export default Task;

interface ButtonProps {
  onClick: (event: React.MouseEvent<HTMLButtonElement>) => void;
}

function StartButton({ onClick }: ButtonProps) {
  return (
    <button
      onClick={onClick}
      className="flex bg-green-500 px-4 py-2 rounded text-white gap-2 cursor-pointer"
    >
      <CirclePlay />
      <span>Start</span>
    </button>
  );
}

function StopButton({ onClick }: ButtonProps) {
  return (
    <button
      onClick={onClick}
      className="flex bg-red-500 px-4 py-2 rounded text-white gap-2 cursor-pointer"
    >
      <StopCircle />
      <span>Stop</span>
    </button>
  );
}
