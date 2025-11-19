import MainLayout from "@/layouts/MainLayout";
import {
  ChevronLeft,
  CircleAlert,
  CircleCheckBig,
  CirclePlay,
  StopCircle,
} from "lucide-react";
import React, { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { intervalToDuration, formatDistance } from "date-fns";
import upworkLogo from "@/assets/images/upwork_logo.png";
import {
  TakeScreenshot,
  GetMonitorCount,
  StartTracking,
  StopTracking,
  GetLastTaskRecord,
  OpenUpwork,
} from "@/../wailsjs/go/app/App";
import { EventsOn } from "@/../wailsjs/runtime";
import { EditWorkDialog } from "@/components/custom/EditWorkDialog";
import { useAuthStore } from "@/store/AuthContext";
import { models } from "wailsjs/go/models";

interface ScreenshotProps {
  date: number;
  pictures: string[];
}

const Task = () => {
  const navigate = useNavigate();
  const { id } = useParams();
  const [isStarted, setStart] = useState(false);
  const [timeStarted, setStartTime] = useState<number>(0);
  const [timeEnded, setEndTime] = useState<number>();
  const [elapsedTime, setElapsedTime] = useState<number>(0);
  const [monitor, setMonitor] = useState<number>(0);
  const [screenshots, setScreenshots] = useState<ScreenshotProps | null>(null);
  const [screenshotDate, setScreenshotDate] = useState<string>("");
  const [memo, setMemo] = useState("");
  const [tag, setTag] = useState("");
  const [taskDetails, setTaskDetails] = useState<models.ExpTask>();
  const [isSynced, setSynced] = useState(false);
  const [lastTaskRecord, setLastTaskRecord] = useState<models.ExpTaskRecord>();
  const [isLaunching, setLaunching] = useState(false);

  const { setTaskID, tasks } = useAuthStore();

  function fetchTaskDetails() {
    tasks.forEach((value) => {
      if (value.taskId === id) setTaskDetails(value);
    });
  }

  async function startTime() {
    resetTimer();
    if (!id) return;
    const timeStart = Date.now();
    const stringifiedDateTime = JSON.stringify(timeStart);
    setStart(true);
    setStartTime(timeStart);
    await StartTracking(id, stringifiedDateTime);
  }

  function resetTimer() {
    setStartTime(0);
    setEndTime(0);
    setElapsedTime(0);
  }

  async function stopTime() {
    setTaskID(null);
    const timeStop = Date.now();
    setStart(false);
    setEndTime(timeStop);
    if (id) await StopTracking(id);
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

  useEffect(() => {
    console.log(lastTaskRecord);
    setSynced(lastTaskRecord?.sync_status === "synced");
  }, [lastTaskRecord]);

  // TODO remove this because testing only
  // async function screenshotNow() {
  //   const now = Date.now();
  //   const pictures = await TakeScreenshot();
  //   if (pictures) {
  //     const screenshotTaken: ScreenshotProps = { date: now, pictures };
  //     localStorage.setItem(
  //       "last_screenshot_" + id,
  //       JSON.stringify(screenshotTaken)
  //     );
  //     setScreenshots(screenshotTaken);
  //   }
  // }

  async function countMonitor() {
    const monitorCount = await GetMonitorCount();
    setMonitor(monitorCount);
  }

  function getLastScreenshotTime() {
    let screenshotTimeAgo = "No screenshots yet.";
    const lastScreenshotDate = screenshots?.date;
    if (lastScreenshotDate) {
      screenshotTimeAgo = formatDistance(lastScreenshotDate, new Date(), {
        addSuffix: true,
      });
    }
    setScreenshotDate(screenshotTimeAgo);
  }

  async function LaunchUpwork() {
    // Open upwork on that account

    if (isLaunching) return;

    if (taskDetails?.taskId) {
      setLaunching(true);
      await OpenUpwork(taskDetails.taskId);
    }
  }

  function openPreview(base64: string) {
    // Create a temporary image to get dimensions
    const width = 1280;
    const height = 800;

    // Center the window
    const left = Math.max((screen.width - width) / 2, 0);
    const top = Math.max((screen.height - height) / 2, 0);

    const newWindow = window.open(
      "",
      "_blank",
      `width=${width},height=${height},left=${left},top=${top}`
    );

    if (newWindow) {
      newWindow.document.title = "Screenshot Preview";
      newWindow.document.body.style.margin = "0";
      newWindow.document.body.style.background = "#000";

      // Create a new Image for the new window
      const img = newWindow.document.createElement("img");
      img.src = base64;
      img.style.width = `${width}px`;
      img.style.height = `${height}px`;

      newWindow.document.body.appendChild(img);
    }
  }

  async function fetchLastRecord() {
    if (!id) return;
    try {
      const response = await GetLastTaskRecord(id);
      if (response) {
        const record = response.record;
        const screenshots = response.screenshots;
        const date = response.takenAt;

        if (record) {
          setLastTaskRecord(record);
          const started = new Date(record.started_at);
          const ended = record.ended_at ? new Date(record.ended_at) : null;
          setStartTime(started.getTime());
          setEndTime(ended?.getTime());
        }

        if (screenshots && date) {
          const screenshotTaken: ScreenshotProps = {
            date,
            pictures: screenshots,
          };
          setScreenshots(screenshotTaken);
        }
      }
    } catch (error) {
      console.log("Error fetching last record: " + error);
    }
  }

  useEffect(() => {
    getLastScreenshotTime();
    fetchLastRecord();
  }, [screenshots]);

  useEffect(() => {
    fetchTaskDetails();
    console.log("Task details: ", taskDetails);
    fetchLastRecord();
    const memoRecord = localStorage.getItem("memo");
    const tagRecord = localStorage.getItem("tag");
    if (memoRecord) setMemo(memoRecord);
    if (tagRecord) setTag(tagRecord);
    countMonitor();
  }, []);

  useEffect(() => {
    if (!isStarted || !timeStarted) return;
    const interval = setInterval(() => {
      const elapsed = Math.floor((Date.now() - timeStarted) / 1000);
      setElapsedTime(elapsed);
    }, 1000);
    return () => clearInterval(interval);
  }, [isStarted]);

  useEffect(() => {
    if (timeStarted && !timeEnded && id) {
      setTaskID(id);
      setStart(true);
    }

    fetchLastRecord();
  }, [timeStarted, timeEnded]);

  useEffect(() => {
    const off = EventsOn("upwork-button", () => {
      console.log("upwork button alright");
      setLaunching(false);
    });

    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("screenshot", (pictures: string[]) => {
      const now = Date.now();
      const screenshotTaken: ScreenshotProps = { date: now, pictures };
      setScreenshots(screenshotTaken);
    });
    return () => off();
  }, []);

  return (
    <MainLayout>
      <div className="p-4">
        <div className="flex justify-between w-full items-center">
          <button
            onClick={() => navigate("/home")}
            className="flex gap-2 font-bold cursor-pointer text-foreground"
          >
            <ChevronLeft />
            Back
          </button>
          {taskDetails?.title?.toLowerCase().includes("upwork") && (
            <img
              onClick={LaunchUpwork}
              src={upworkLogo}
              alt=""
              className={`h-10 cursor-pointer rounded-full hover:shadow-[0_0_10px_1px] shadow-green-500 transition-shadow ${
                isLaunching && "opacity-50 pointer-none cursor-not-allowed"
              }`}
            />
          )}
        </div>
        <h1 className="text-2xl font-bold mt-2 line-clamp-2 text-foreground">
          {taskDetails?.title}
        </h1>
        <div className="flex flex-col mt-1">
          <span className="font-extrabold text-gray-600 dark:text-gray-300">
            {taskDetails?.client}
          </span>
          <span className="text-[10px] text-gray-500 dark:text-gray-400 mt-[-6px]">
            Client
          </span>
        </div>
      </div>

      <hr className="border-gray-200 dark:border-gray-700" />

      <div className="p-5">
        <div className="rounded border border-gray-300 dark:border-gray-700 p-4 bg-white dark:bg-neutral-900">
          <span className="font-bold text-foreground">
            {isStarted ? "Current" : "Last"} session
          </span>

          <div className="flex justify-between items-center">
            <div className="flex flex-col">
              <span className="font-bold text-2xl text-foreground">
                {getFormattedTime()}
              </span>
              <span className="text-sm text-gray-600 dark:text-gray-400">
                Time tracking {isStarted ? "on" : "off"}
              </span>
            </div>
            <div className="flex flex-col gap-2">
              {isSynced ? (
                <div className="flex gap-1 items-center text-xs text-green-500">
                  Synced <CircleCheckBig size={12} />
                </div>
              ) : (
                <div className="flex gap-1 items-center text-xs text-amber-500">
                  Not Synced <CircleAlert size={12} />
                </div>
              )}
              {isStarted ? (
                <StopButton onClick={() => stopTime()} />
              ) : (
                <StartButton onClick={() => startTime()} />
              )}
            </div>
          </div>

          <hr className="my-2 border-gray-200 dark:border-gray-700" />

          <div className="flex justify-between items-center">
            <h3 className="text-base font-semibold text-foreground">
              What are you working on?
            </h3>
            <EditWorkDialog
              initialMemo={memo}
              initialTag={tag}
              onSave={(newMemo, newTag) => {
                setMemo(newMemo);
                setTag(newTag);
                localStorage.setItem("memo", newMemo);
                localStorage.setItem("tag", newTag);
              }}
            />
          </div>

          <div className="space-y-1 text-sm">
            <p>
              <span className="font-medium text-gray-700 dark:text-gray-300">
                📌 <strong>Memo:</strong>
              </span>{" "}
              <span className="truncate">{memo || "No memo set."}</span>
            </p>
            <p className="line-clamp-3">
              <span className="font-medium text-gray-700 dark:text-gray-300">
                🏷️ <strong>Activity tag:</strong>
              </span>{" "}
              {tag || "No tag set."}
            </p>
          </div>
        </div>

        <div className="flex flex-col mt-3 mb-1">
          <div className="flex justify-between">
            <h2 className="font-bold text-foreground">Latest screenshot</h2>
            {/* <button
              onClick={screenshotNow}
              className="px-3 py-1 rounded bg-green-500 text-white hover:bg-green-600 transition"
            >
              Screenshot
            </button> */}
          </div>
          <div className="flex justify-between items-baseline text-sm">
            <span className="text-gray-600 dark:text-gray-400">
              Active monitors: {monitor}
            </span>
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {screenshotDate}
            </span>
          </div>
        </div>

        <div>
          {screenshots &&
            screenshots.pictures.map((value, index) => (
              <img
                key={index}
                src={value}
                alt=""
                className="w-full border rounded-2xl mb-2 object-cover border-gray-300 dark:border-gray-700"
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
      className="flex bg-green-500 px-4 py-2 rounded text-white gap-2 cursor-pointer hover:bg-green-600 transition"
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
      className="flex bg-red-500 px-4 py-2 rounded text-white gap-2 cursor-pointer hover:bg-red-600 transition"
    >
      <StopCircle />
      <span>Stop</span>
    </button>
  );
}
