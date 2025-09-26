import MainLayout from "@/layouts/MainLayout";
import { ChevronLeft, CirclePlay, StopCircle } from "lucide-react";
import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { intervalToDuration, formatDistance } from "date-fns";
import {
  TakeScreenshot,
  GetMonitorCount,
  StartTracking,
  StopTracking,
} from "@/../wailsjs/go/app/App";
import { EventsOn } from "@/../wailsjs/runtime";
import { EditWorkDialog } from "@/components/custom/EditWorkDialog";

interface ScreenshotProps {
  date: number;
  pictures: string[];
}

const Task = () => {
  const navigate = useNavigate();
  const [isStarted, setStart] = useState(false);
  const [timeStarted, setStartTime] = useState<number>(0);
  const [timeEnded, setEndTime] = useState<number>();
  const [elapsedTime, setElapsedTime] = useState<number>(0);
  const [monitor, setMonitor] = useState<number>(0);
  const [screenshots, setScreenshots] = useState<ScreenshotProps | null>(null);
  const [screenshotDate, setScreenshotDate] = useState<string>("");
  const [memo, setMemo] = useState("");
  const [tag, setTag] = useState("");

  function startTime() {
    resetTimer();

    const timeStart = Date.now();
    const stringifiedDateTime = JSON.stringify(timeStart);
    setStart(true);
    localStorage.setItem("started", stringifiedDateTime);
    setStartTime(timeStart);
    StartTracking(stringifiedDateTime);
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
    StopTracking();
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
    const now = Date.now();
    const pictures = await TakeScreenshot();

    if (pictures) {
      const screenshotTaken: ScreenshotProps = { date: now, pictures };

      localStorage.setItem("last_screenshot", JSON.stringify(screenshotTaken));
      setScreenshots(screenshotTaken);
    }
  }

  async function countMonitor() {
    const monitorCount = await GetMonitorCount();
    setMonitor(monitorCount);
  }

  function getLastScreenshotTime() {
    let screenshotTimeAgo = "No screenshots yet.";

    const lastScreenshotDate = screenshots?.date;
    if (lastScreenshotDate) {
      // console.log("nisud diri");
      screenshotTimeAgo = formatDistance(lastScreenshotDate, new Date(), {
        addSuffix: true,
      });
      console.log(screenshotTimeAgo);
    }

    setScreenshotDate(screenshotTimeAgo);
  }

  function openPreview(base64: string) {
    const newWindow = window.open(
      "",
      "_blank",
      `width=200,height=200,resizable=yes,left=0,top=500`
    );
    if (newWindow) {
      const img = newWindow.document.createElement("img");
      img.src = base64;
      img.style.objectPosition = "cover";
      img.style.height = "auto";
      img.style.width = "1280";

      newWindow.document.body.style.margin = "0"; // remove margins
      newWindow.document.body.appendChild(img);

      img.onload = () => {
        const w = img.width;
        const h = img.height;
        newWindow.resizeTo(w + 20, h + 40);
        // +20/+40 padding for window chrome/borders
      };
    }
  }

  useEffect(() => {
    getLastScreenshotTime();
  }, [screenshots]);

  useEffect(() => {
    const timeStartRecord = localStorage.getItem("started");
    const timeStopRecord = localStorage.getItem("ended");
    const screenshotRecord = localStorage.getItem("last_screenshot");

    if (timeStartRecord) setStartTime(JSON.parse(timeStartRecord));
    if (timeStopRecord) setEndTime(JSON.parse(timeStopRecord));
    if (screenshotRecord) {
      const record = JSON.parse(screenshotRecord);
      setScreenshots(record);
    }

    const memoRecord = localStorage.getItem("memo");
    const tagRecord = localStorage.getItem("tag");
    if (memoRecord) setMemo(memoRecord);
    if (tagRecord) setTag(tagRecord);

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

  useEffect(() => {
    const off = EventsOn("screenshot", (pictures: string[]) => {
      const now = Date.now();
      const screenshotTaken: ScreenshotProps = { date: now, pictures };
      setScreenshots(screenshotTaken);
    });

    return () => {
      console.log("app is unmounted");
      off();
    };
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
          <span className="font-bold">
            {isStarted ? "Current" : "Last"} session
          </span>
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
          <div className="flex flex-col">
            <span className="text-sm max-w-xs truncate">
              <strong>Memo:</strong> {memo || "No memo set."}
            </span>

            <span className="text-sm max-w-md line-clamp-3">
              <strong>Activity tag:</strong> {tag || "No tag set."}
            </span>
          </div>
        </div>

        <div className="flex flex-col mt-3 mb-1">
          <div className="flex justify-between">
            <h2 className="font-bold">Latest screenshot</h2>
            <button onClick={screenshotNow}>Screenshot</button>
          </div>
          <div className="flex justify-between items-baseline">
            <span>Active monitors: {monitor}</span>
            <span className="text-xs">{screenshotDate}</span>
          </div>
        </div>
        <div>
          {screenshots &&
            screenshots.pictures.map((value, index) => (
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
