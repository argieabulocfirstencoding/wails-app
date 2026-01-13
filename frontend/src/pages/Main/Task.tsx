import MainLayout from "@/layouts/MainLayout";
import {
  ChevronLeft,
  CircleAlert,
  CircleCheckBig,
  CirclePlay,
  StopCircle,
  RefreshCcw,
} from "lucide-react";
import React, { forwardRef, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { intervalToDuration, formatDistance } from "date-fns";
import upworkLogo from "@/assets/images/upwork_logo.png";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  TakeScreenshot,
  GetMonitorCount,
  StartTracking,
  StopTracking,
  GetLastTaskRecord,
  AddMemo,
  OpenUpwork,
  SyncUnsyncedTaskRecord,
} from "@/../wailsjs/go/app/App";
import { EventsOn } from "@/../wailsjs/runtime";
import { EditWorkDialog } from "@/components/custom/EditWorkDialog";
import { useAuthStore } from "@/store/AuthContext";
import { models } from "wailsjs/go/models";
import SpinnerLoader from "@/components/custom/SpinnerLoader";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { DialogClose } from "@radix-ui/react-dialog";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemTitle,
} from "@/components/ui/item";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Label } from "@/components/ui/label";

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
  const [isSyncing, setSyncing] = useState(false);
  const [isPadagan, setPadagan] = useState(false);

  const { setTaskID, tasks } = useAuthStore();

  function fetchTaskDetails() {
    tasks.forEach((value) => {
      if (value.taskId === id) setTaskDetails(value);
    });
  }

  async function startTime() {
    setSyncing(true);
    resetTimer();
    if (!id) return;
    const timeStart = Date.now();
    const stringifiedDateTime = JSON.stringify(timeStart);
    setStart(true);
    setStartTime(timeStart);
    await StartTracking(id, stringifiedDateTime, isPadagan).then(() => {
      setTimeout(() => {
        setSyncing(false);
      }, 1000);
    });
  }

  function resetTimer() {
    setStartTime(0);
    setEndTime(0);
    setElapsedTime(0);
  }

  async function stopTime() {
    setSyncing(true);
    if (id) await StopTracking(id);
    (async () => {
      await fetchLastRecord();
    })();

    // await fetchLastRecord();
    setTaskID(null);
    const timeStop = Date.now();
    setStart(false);
    setEndTime(timeStop);

    setTimeout(() => {
      setSyncing(false);
    }, 1500);
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
  }, [lastTaskRecord?.sync_status]);

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

  async function handleSyncTaskRecord() {
    if (!lastTaskRecord) return;
    setSyncing(true);
    await SyncUnsyncedTaskRecord(lastTaskRecord.id);
    (async () => {
      await fetchLastRecord();
    })();

    setTimeout(() => {
      setSyncing(false);
    }, 1500);
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

          console.log("RECORD PADAGAN: ", record.padagan);
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
  }, [isSyncing]);

  useEffect(() => {
    fetchTaskDetails();
    console.log("Task details: ", taskDetails);
    const memoRecord = localStorage.getItem("memo");
    const tagRecord = localStorage.getItem("tag");
    if (memoRecord) setMemo(memoRecord);
    if (tagRecord) setTag(tagRecord);

    AddMemo(memoRecord || "", tagRecord || "");
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
      setSyncing(true);
      const now = Date.now();
      const screenshotTaken: ScreenshotProps = { date: now, pictures };
      setScreenshots(screenshotTaken);

      fetchLastRecord();
      setTimeout(() => {
        setSyncing(false);
      }, 1500);
    });
    return () => off();
  }, []);

  return (
    <MainLayout>
      {/* Sticky top bar: Back + Upwork */}
      <div
        className="
        sticky top-0 z-30
        bg-white/90 dark:bg-neutral-950/85
        backdrop-blur-md
        border-b border-neutral-200 dark:border-neutral-800
        shadow-sm
      "
      >
        <div className="max-w-3xl mx-auto px-4 py-3 flex items-center justify-between">
          <button
            onClick={() => navigate("/home")}
            className="
            group inline-flex items-center gap-1.5 text-sm font-semibold
            text-neutral-800 dark:text-neutral-100
            hover:text-emerald-500 dark:hover:text-emerald-400
            transition-colors cursor-pointer
          "
          >
            <ChevronLeft className="w-4 h-4 transition-transform group-hover:-translate-x-0.5" />
            <span>Back</span>
          </button>

          {taskDetails?.title?.toLowerCase().includes("upwork") &&
            !taskDetails?.title?.toLowerCase().includes("training") && (
              <img
                onClick={LaunchUpwork}
                src={upworkLogo}
                alt="Open in Upwork"
                className={`h-9 w-9 rounded-full cursor-pointer flex-shrink-0
                border-2 border-transparent dark:border-emerald-500
                shadow-sm transition-all
                hover:shadow-[0_0_12px_1px_rgba(16,185,129,0.7)]
                hover:border-emerald-500
                ${
                  isLaunching
                    ? "opacity-50 pointer-events-none cursor-not-allowed"
                    : ""
                }`}
              />
            )}
        </div>
      </div>

      {/* Title + client (NON-sticky, just under header) */}
      <div className="max-w-3xl mx-auto px-4 pt-3 pb-1">
        <h1 className="text-lg sm:text-xl md:text-2xl font-bold leading-snug line-clamp-2 text-neutral-900 dark:text-white">
          {taskDetails?.title}
        </h1>

        <div className="mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <div>
            <span className="block text-sm font-semibold text-neutral-800 dark:text-gray-200">
              {taskDetails?.client}
            </span>
            <span
              className="text-[11px] px-2 py-0.5 rounded-full 
              bg-neutral-200 dark:bg-slate-800 
              text-neutral-700 dark:text-gray-200"
            >
              Client
            </span>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="max-w-3xl mx-auto px-4 py-5">
        <div className="rounded-2xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 px-4 sm:px-5 py-4 sm:py-5 shadow-sm">
          {/* Header: label + sync status */}
          <header className="flex items-center justify-between gap-3">
            <span className="text-[11px] sm:text-xs font-semibold tracking-[0.16em] uppercase text-neutral-500 dark:text-neutral-400">
              {isStarted ? "Current session" : "Last session"}
            </span>
          </header>

          {/* Time + Start/Stop */}
          <div className="flex items-end justify-between gap-4">
            <div className="space-y-1">
              <p className="text-2xl sm:text-3xl font-semibold text-neutral-900 dark:text-neutral-50">
                {getFormattedTime()}
              </p>
              <p className="text-xs sm:text-sm text-neutral-600 dark:text-neutral-400">
                Time tracking {isStarted ? "on" : "off"}
              </p>
              <p
                className={`text-xs sm:text-sm  ${
                  lastTaskRecord?.padagan
                    ? "text-red-500"
                    : "text-neutral-600 dark:text-neutral-400"
                } `}
              >
                {lastTaskRecord?.padagan ? "Padagan" : ""}
              </p>
            </div>

            <div className="flex flex-col items-end gap-2">
              <div className="flex items-center gap-2 text-[11px] sm:text-xs">
                {isSyncing ? (
                  <span className="flex items-center gap-1 text-neutral-500 dark:text-neutral-400">
                    Syncing
                    <SpinnerLoader className="scale-[0.45]" />
                  </span>
                ) : isSynced ? (
                  <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                    Synced
                    <CircleCheckBig className="w-3 h-3" />
                  </span>
                ) : (
                  lastTaskRecord && (
                    <span className="inline-flex items-center gap-1 text-amber-500">
                      Not synced
                      <CircleAlert className="w-3 h-3" />
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            onClick={handleSyncTaskRecord}
                            className="ml-1 rounded-full p-1 text-neutral-400 dark:text-neutral-300 hover:text-neutral-700 dark:hover:text-white transition-colors cursor-pointer"
                          >
                            <RefreshCcw className="w-3.5 h-3.5" />
                          </button>
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>Click to sync this session</p>
                        </TooltipContent>
                      </Tooltip>
                    </span>
                  )
                )}
              </div>
              {taskDetails?.title.toLowerCase().includes("break") ||
              taskDetails?.title.toLowerCase().includes("training") ? (
                isStarted ? (
                  <StopButton onClick={stopTime} />
                ) : (
                  <StartButton onClick={startTime} />
                )
              ) : isStarted ? (
                <StopButton onClick={stopTime} />
              ) : (
                <Dialog>
                  <DialogTrigger asChild>
                    <StartButton />
                  </DialogTrigger>
                  <DialogContent className="sm:max-w-[425px]">
                    <DialogHeader>
                      <DialogTitle>Select Tracking Type</DialogTitle>
                      <DialogDescription>
                        Choose between if this task is for padagan or a direct
                        task tracking.
                      </DialogDescription>
                    </DialogHeader>
                    <RadioGroup
                      defaultValue={isPadagan ? "Padagan" : "Not Padagan"}
                      className="grid gap-3"
                      onValueChange={(v) => {
                        console.log("Padagan: ", v === "Padagan");
                        if (v === "Padagan") {
                          setPadagan(true);
                        } else {
                          setPadagan(false);
                        }
                      }}
                    >
                      <Label htmlFor="padagan" className="cursor-pointer">
                        <Item
                          variant="outline"
                          className="transition-all rounded-lg border 
                  has-[button[data-state=checked]]:border-amber-500
                  has-[button[data-state=checked]]:ring-2
                  has-[button[data-state=checked]]:ring-amber-500/40"
                        >
                          <ItemContent>
                            <ItemTitle className="text-amber-500">
                              Padagan
                            </ItemTitle>
                            <ItemDescription>
                              Indirect task; work now, padagan later.
                            </ItemDescription>
                          </ItemContent>

                          <ItemActions>
                            <RadioGroupItem id="padagan" value="Padagan" />
                          </ItemActions>
                        </Item>
                      </Label>

                      <Label htmlFor="notpadagan" className="cursor-pointer">
                        <Item
                          variant="outline"
                          className="
                            transition-all
                            has-[button[data-state=checked]]:border-emerald-500
                            has-[button[data-state=checked]]:ring-2
                            has-[button[data-state=checked]]:ring-emerald-500/40
                          "
                        >
                          <ItemContent>
                            <ItemTitle className="text-emerald-500">
                              Regular Task
                            </ItemTitle>
                            <ItemDescription>
                              Client tracking is active while working.
                            </ItemDescription>
                          </ItemContent>

                          <ItemActions>
                            <RadioGroupItem
                              id="notpadagan"
                              value="Not Padagan"
                            />
                          </ItemActions>
                        </Item>
                      </Label>
                    </RadioGroup>

                    <DialogFooter>
                      <DialogClose asChild>
                        <Button variant="outline">Cancel</Button>
                      </DialogClose>
                      <Button type="submit" onClick={startTime}>
                        Start
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              )}
            </div>
          </div>

          {!taskDetails?.title.toLowerCase().includes("break") ? (
            <>
              {/* Divider */}
              <div className="mt-4 border-t border-neutral-200 dark:border-neutral-700 pt-4" />

              {/* What are you working on? */}
              <div className="flex items-center justify-between gap-2 mb-2">
                <h3 className="text-sm sm:text-base font-semibold text-neutral-900 dark:text-neutral-50">
                  What are you working on?
                </h3>
                <EditWorkDialog
                  initialMemo={memo}
                  initialTag={tag}
                  onSave={(newMemo, newTag) => {
                    async function updateMemo() {
                      setMemo(newMemo);
                      setTag(newTag);
                      localStorage.setItem("memo", newMemo);
                      localStorage.setItem("tag", newTag);

                      await AddMemo(newMemo || "", newTag || "");
                    }

                    updateMemo();
                  }}
                />
              </div>

              <div className="space-y-1 text-xs sm:text-sm">
                <p className="flex gap-1.5">
                  <span className="font-medium text-neutral-700 dark:text-neutral-300">
                    📌 <strong>Memo:</strong>
                  </span>
                  <span className="truncate text-neutral-800 dark:text-neutral-50">
                    {memo || "No memo set."}
                  </span>
                </p>
                <p className="flex gap-1.5 line-clamp-3">
                  <span className="font-medium text-neutral-700 dark:text-neutral-300">
                    🏷️ <strong>Activity tag:</strong>
                  </span>
                  <span className="text-neutral-800 dark:text-neutral-50">
                    {tag || "No tag set."}
                  </span>
                </p>
              </div>
            </>
          ) : (
            <div className="rounded-xl overflow-hidden mt-12">
              <img
                src="https://storage.gra.cloud.ovh.net/v1/AUTH_728ab22ba0c842468a897907be9a7075/flipanim/anim/uvvk8grs.gif"
                alt=""
              />
            </div>
          )}
        </div>

        {/* Latest screenshot block + images stay the same */}
        {!taskDetails?.title.toLowerCase().includes("break") ? (
          <>
            <div className="flex flex-col mt-4 mb-1">
              <div className="flex justify-between items-baseline">
                <h2 className="text-sm sm:text-base font-semibold text-foreground">
                  Latest screenshot
                </h2>
              </div>
              <div className="flex justify-between items-baseline text-xs sm:text-sm mt-1">
                <span className="text-gray-600 dark:text-gray-400">
                  Active monitors: {monitor}
                </span>
                <span className="text-[11px] text-gray-500 dark:text-gray-400">
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
                    className="w-full border rounded-2xl mb-2 object-cover border-gray-300 dark:border-gray-700 cursor-pointer hover:brightness-105 transition"
                    onClick={() => openPreview(value)}
                  />
                ))}
            </div>
          </>
        ) : (
          <></>
        )}
      </div>
    </MainLayout>
  );
};

export default Task;

interface ButtonProps {
  onClick: (event: React.MouseEvent<HTMLButtonElement>) => void;
}

const StartButton = forwardRef<
  HTMLButtonElement,
  React.ComponentProps<typeof Button>
>((props, ref) => {
  return (
    <Button
      ref={ref}
      {...props}
      className="
        inline-flex items-center gap-1 px-4 py-2 rounded-full
        bg-emerald-500 text-white text-sm font-semibold
        shadow-sm
        hover:bg-emerald-600 active:bg-emerald-700
        active:scale-[0.98]
        transition-all duration-150
        cursor-pointer
      "
    >
      <CirclePlay className="w-4 h-4" />
      <span>Start</span>
    </Button>
  );
});

function StopButton({ onClick }: ButtonProps) {
  return (
    <button
      onClick={onClick}
      className="
        inline-flex items-center gap-1
        px-4 py-2 rounded-full
        bg-red-500 text-white text-sm font-semibold
        shadow-sm
        hover:bg-red-600 active:bg-red-700
        active:scale-[0.98]
        transition-all duration-150
        cursor-pointer
      "
    >
      <StopCircle className="w-4 h-4" />
      <span>Stop</span>
    </button>
  );
}
