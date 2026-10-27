import MainLayout from "@/layouts/MainLayout";
import {
  ChevronLeft,
  CircleAlert,
  CircleCheckBig,
  CirclePlay,
  StopCircle,
} from "lucide-react";
import React, { forwardRef, useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { formatDistance } from "date-fns";
import upworkLogo from "@/assets/images/upwork_logo.png";
import {
  GetMonitorCount,
  StartTracking,
  StopTrackingRecord,
  GetLastTaskRecord,
  GetTaskShiftTime,
  AddMemo,
  OpenUpwork,
  SetTaskRecordNotes,
} from "@/../wailsjs/go/app/App";
import { BrowserOpenURL, EventsOn } from "@/../wailsjs/runtime";
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
import { releaseFlags } from "@/config/release";

interface ScreenshotProps {
  date: number;
  pictures: string[];
}

type UpworkLoginStatus = {
  mode?: string;
  state?: string;
  message?: string;
};

type TrackingAction = "idle" | "starting" | "stopping";

const Task = () => {
  const navigate = useNavigate();
  const { id } = useParams();
  const [isStarted, setStart] = useState(false);
  const [currentSessionStartedAt, setCurrentSessionStartedAt] = useState(0);
  const [completedShiftSeconds, setCompletedShiftSeconds] = useState(0);
  const [timerTick, setTimerTick] = useState(Date.now());
  const [monitor, setMonitor] = useState<number>(0);
  const [screenshots, setScreenshots] = useState<ScreenshotProps | null>(null);
  const [screenshotDate, setScreenshotDate] = useState<string>("");
  const [notes, setNotes] = useState("");
  const [taskDetails, setTaskDetails] = useState<models.ExpTask>();
  const [isSynced, setSynced] = useState(false);
  const [lastTaskRecord, setLastTaskRecord] = useState<models.ExpTaskRecord>();
  const [isLaunching, setLaunching] = useState(false);
  const [upworkNotice, setUpworkNotice] = useState("");
  const [isSyncing, setSyncing] = useState(false);
  const [isPadagan, setPadagan] = useState(false);
  const [isHydrated, setHydrated] = useState(false);
  const [syncNotice, setSyncNotice] = useState("");
  const [trackingAction, setTrackingAction] =
    useState<TrackingAction>("idle");
  const actionRef = useRef<TrackingAction>("idle");
  const hydrationRequestRef = useRef(0);
  const pendingHydrationRef = useRef(false);

  const { setTaskID, tasks, online, synced } = useAuthStore();

  function beginTrackingAction(action: Exclude<TrackingAction, "idle">) {
    if (actionRef.current !== "idle") return false;
    actionRef.current = action;
    setTrackingAction(action);
    return true;
  }

  async function finishTrackingAction() {
    actionRef.current = "idle";
    setTrackingAction("idle");
    if (pendingHydrationRef.current) {
      pendingHydrationRef.current = false;
      await hydrateTaskRecord();
    }
  }

  async function startTime() {
    if (!id || !isHydrated || !beginTrackingAction("starting")) return;

    try {
      setSyncing(true);

      const timeStart = Date.now();
      await StartTracking(id, JSON.stringify(timeStart), isPadagan);

      await hydrateTaskRecord(true);
    } catch (error) {
      console.error("StartTracking failed:", error);
    } finally {
      setSyncing(false);
      await finishTrackingAction();
    }
  }

  async function stopTime() {
    const taskRecordID = lastTaskRecord?.id;
    if (!taskRecordID || !beginTrackingAction("stopping")) return;

    setSyncing(true);
    setSyncNotice("");
    try {
      const result = await StopTrackingRecord(taskRecordID);
      setSynced(result.syncStatus === "synced");

      if (result.syncFailureCode === "session_expired") {
        setSyncNotice(
          "Time was saved locally. Your Bubble session expired—use Sync to sign in again and upload it.",
        );
      } else if (result.syncFailureCode === "offline") {
        setSyncNotice(
          "Time was saved locally. Connect to the internet and use Sync to upload it.",
        );
      } else if (result.syncFailureCode === "sync_failed") {
        setSyncNotice(
          "Time was saved locally, but Bubble could not be updated. Use Sync to retry it.",
        );
      }

      setTaskID(null);
      await hydrateTaskRecord(true);
    } catch (error) {
      console.error("StopTracking failed:", error);
      setSyncNotice("The task could not be stopped locally. Please try again.");
    } finally {
      setSyncing(false);
      await finishTrackingAction();
    }
  }

  function formatDuration(seconds: number) {
    const safeSeconds = Math.max(0, Math.floor(seconds));
    const hours = Math.floor(safeSeconds / 3600);
    const minutes = Math.floor((safeSeconds % 3600) / 60);
    const remainingSeconds = safeSeconds % 60;
    return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(
      2,
      "0",
    )}:${String(remainingSeconds).padStart(2, "0")}`;
  }

  async function countMonitor() {
    const monitorCount = await GetMonitorCount();
    setMonitor(monitorCount);
  }

  async function hydrateTaskRecord(allowDuringAction = false) {
    if (!id) return;
    if (actionRef.current !== "idle" && !allowDuringAction) {
      pendingHydrationRef.current = true;
      return;
    }

    const requestID = ++hydrationRequestRef.current;

    try {
      const [response, shiftTime] = await Promise.all([
        GetLastTaskRecord(id),
        GetTaskShiftTime(id),
      ]);
      if (
        requestID !== hydrationRequestRef.current ||
        (actionRef.current !== "idle" && !allowDuringAction)
      ) {
        pendingHydrationRef.current = true;
        return;
      }
      const record = response?.record;
      const activeStartedAt = shiftTime?.current_session_started_at
        ? new Date(shiftTime.current_session_started_at).getTime()
        : 0;
      const hasActiveSession = Number.isFinite(activeStartedAt) && activeStartedAt > 0;

      setCurrentSessionStartedAt(hasActiveSession ? activeStartedAt : 0);
      setCompletedShiftSeconds(
        Math.max(0, Number(shiftTime?.completed_shift_seconds ?? 0)),
      );
      setStart(hasActiveSession);
      setTaskID(hasActiveSession ? (id ?? null) : null);

      if (!record || !record.started_at) {
        setLastTaskRecord(undefined);
        setSynced(false);
        setScreenshots(null);
        setNotes("");
        return;
      }

      setLastTaskRecord(record);
      setSynced(record.sync_status === "synced");
      setNotes(record.notes ?? "");

      if (
        releaseFlags.screenshotsEnabled &&
        response.screenshots &&
        response.takenAt
      ) {
        setScreenshots({
          date: response.takenAt,
          pictures: response.screenshots,
        });
      } else {
        setScreenshots(null);
      }
    } catch (err) {
      console.error("Refresh error:", err);
      setCurrentSessionStartedAt(0);
      setCompletedShiftSeconds(0);
      setStart(false);
      setTaskID(null);
    } finally {
      setHydrated(true);
    }
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

    if (isLaunching || !online) return;

    if (taskDetails?.taskId) {
      setLaunching(true);
      setUpworkNotice("");
      try {
        await OpenUpwork(taskDetails.taskId);
      } catch (error) {
        console.error("Failed to launch Upwork automation:", error);
        setLaunching(false);
        setUpworkNotice("Unable to start Upwork sign-in. Please try again.");
      }
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
      `width=${width},height=${height},left=${left},top=${top}`,
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

  useEffect(() => {
    if (!id) return;
    const task = tasks.find((t) => t.taskId === id);
    if (task) setTaskDetails(task);
  }, [tasks, id]);

  useEffect(() => {
    if (!releaseFlags.screenshotsEnabled) {
      setScreenshotDate("Screenshot capture is disabled in this release.");
      return;
    }

    getLastScreenshotTime();
  }, [screenshots]);

  useEffect(() => {
    setSynced(lastTaskRecord?.sync_status === "synced");
  }, [lastTaskRecord?.sync_status]);

  useEffect(() => {
    const noteRecord = localStorage.getItem("tag");
    if (noteRecord) setNotes(noteRecord);

    AddMemo(noteRecord || ""); // FIX make the AddMemo to only 1 params
    countMonitor();
  }, []);

  useEffect(() => {
    if (!isStarted || !currentSessionStartedAt) return;
    setTimerTick(Date.now());
    const interval = setInterval(() => {
      setTimerTick(Date.now());
    }, 1000);
    return () => clearInterval(interval);
  }, [isStarted, currentSessionStartedAt]);

  useEffect(() => {
    hydrateTaskRecord();
  }, [id]);

  useEffect(() => {
    if (!EventsOn) return;
    const off = EventsOn("upwork-login-status", (payload: UpworkLoginStatus) => {
      if (payload?.mode !== "task") return;
      if (payload.state === "succeeded") {
        setUpworkNotice("");
      }
      if (payload.state === "failed" || payload.state === "cancelled") {
        setUpworkNotice(payload.message || "Upwork sign-in did not complete.");
      }
      if (payload.state === "succeeded" || payload.state === "failed" || payload.state === "cancelled") {
        setLaunching(false);
      }
    });

    return () => off();
  }, []);

  useEffect(() => {
    (async function () {
      if (lastTaskRecord && isStarted) {
        await SetTaskRecordNotes(lastTaskRecord.id, notes);
      }
    })();
  }, [notes]);

  useEffect(() => {
    if (!releaseFlags.screenshotsEnabled || !EventsOn) return;
    const off = EventsOn("screenshot", (pictures: string[]) => {
      setSyncing(true);
      const now = Date.now();
      const screenshotTaken: ScreenshotProps = { date: now, pictures };
      setScreenshots(screenshotTaken);
    });
    return () => off();
  }, []);

  useEffect(() => {
    if (!EventsOn) return;
    const off = EventsOn("record-synced", async (synced: boolean) => {
      setSynced(synced);
      setSyncing(false);
      await hydrateTaskRecord();
    });

    return () => off();
  }, [id]);

  useEffect(() => {
    if (!EventsOn) return;
    const off = EventsOn("browser-records-synced", async () => {
      await hydrateTaskRecord();
    });

    return () => off();
  }, [id]);
  const titleLower = taskDetails?.title?.toLowerCase() ?? "";
  const currentSessionSeconds = currentSessionStartedAt
    ? Math.max(0, Math.floor((timerTick - currentSessionStartedAt) / 1000))
    : 0;
  const projectShiftSeconds = completedShiftSeconds + currentSessionSeconds;

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
        <div className="max-w-3xl h-[60px] mx-auto px-4 py-3 flex items-center justify-between">
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

          {synced &&
            titleLower.includes("upwork") &&
            !titleLower.includes("training") && (
              <div className="flex items-center gap-2">
                <span
                  className={`max-w-44 text-right text-xs ${upworkNotice ? "text-amber-600 dark:text-amber-400" : "text-neutral-500 dark:text-neutral-400"}`}
                >
                  {upworkNotice || "Press Esc to stop login automation"}
                </span>
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
              </div>
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
          {/* Current-session time + shift total */}
          <header className="flex items-center justify-between gap-3">
            <span className="text-[11px] sm:text-xs font-semibold tracking-[0.16em] uppercase text-neutral-500 dark:text-neutral-400">
              Current session
            </span>
          </header>

          {/* Time + Start/Stop */}
          <div className="flex items-end justify-between gap-4">
            <div className="space-y-1">
              {!isHydrated ? (
                <div className="h-9 w-32 rounded-md bg-neutral-200 dark:bg-neutral-800 animate-pulse" />
              ) : (
                <p className="text-2xl sm:text-3xl font-semibold text-neutral-900 dark:text-neutral-50">
                  {formatDuration(currentSessionSeconds)}
                </p>
              )}
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
              <div className="pt-3 mt-3 border-t border-neutral-200 dark:border-neutral-700">
                <p className="text-[11px] sm:text-xs font-semibold tracking-[0.12em] uppercase text-neutral-500 dark:text-neutral-400">
                  Project time this shift
                </p>
                <p className="mt-1 text-lg sm:text-xl font-semibold text-neutral-900 dark:text-neutral-50">
                  {formatDuration(projectShiftSeconds)}
                </p>
              </div>
            </div>

            <div className="flex flex-col items-end gap-2">
              <div className="flex items-center gap-2 text-[11px] sm:text-xs">
                {isHydrated ? (
                  isSyncing ? (
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
                      </span>
                    )
                  )
                ) : (
                  ""
                )}
              </div>

              {syncNotice && (
                <p className="max-w-56 text-right text-[11px] leading-4 text-amber-600 dark:text-amber-400">
                  {syncNotice}
                </p>
              )}

              {titleLower.includes("break") ||
              titleLower.includes("training") ? (
                isStarted ? (
                  <StopButton
                    onClick={stopTime}
                    disabled={
                      trackingAction !== "idle" || !lastTaskRecord?.id
                    }
                    label={
                      trackingAction === "stopping" ? "Stopping…" : "Stop"
                    }
                  />
                ) : (
                  <StartButton
                    onClick={startTime}
                    disabled={trackingAction !== "idle" || !isHydrated}
                    label={
                      trackingAction === "starting" ? "Starting…" : "Start"
                    }
                  />
                )
              ) : isStarted ? (
                <StopButton
                  onClick={stopTime}
                  disabled={trackingAction !== "idle" || !lastTaskRecord?.id}
                  label={trackingAction === "stopping" ? "Stopping…" : "Stop"}
                />
              ) : (
                <Dialog>
                  <DialogTrigger asChild>
                    <StartButton
                      disabled={trackingAction !== "idle" || !isHydrated}
                      label={
                        trackingAction === "starting" ? "Starting…" : "Start"
                      }
                    />
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
                        if (v === "Padagan") {
                          setPadagan(true);
                        } else {
                          setPadagan(false);
                        }

                        console.log("changed padagan");
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
                      <DialogClose asChild>
                        <Button
                          type="submit"
                          onClick={startTime}
                          disabled={trackingAction !== "idle" || !isHydrated}
                        >
                          {trackingAction === "starting" ? "Starting…" : "Start"}
                        </Button>
                      </DialogClose>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              )}
            </div>
          </div>

          {!titleLower.includes("break") ? (
            <>
              {/* Divider */}
              <div className="mt-4 border-t border-neutral-200 dark:border-neutral-700 pt-4" />

              {/* What are you working on? */}
              <div className="flex items-center justify-between gap-2 mb-2">
                <h3 className="text-sm sm:text-base font-semibold text-neutral-900 dark:text-neutral-50">
                  What are you working on?
                </h3>
                <EditWorkDialog
                  initialNotes={notes}
                  onSave={(newNotes) => {
                    async function updateMemo() {
                      setNotes(newNotes);

                      await AddMemo(newNotes || "");
                    }

                    updateMemo();
                  }}
                />
              </div>

              <div className="space-y-1 text-xs sm:text-sm">
                <p className="flex gap-1.5 line-clamp-3">
                  <span className="font-medium text-neutral-700 dark:text-neutral-300">
                    Activity notes:
                  </span>
                  <span className="text-neutral-800 dark:text-neutral-50">
                    {notes || "No notes set."}
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
        {releaseFlags.screenshotsEnabled && !titleLower.includes("break") ? (
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
  disabled?: boolean;
  label?: string;
}

type StartButtonProps = React.ComponentProps<typeof Button> & {
  label?: string;
};

const StartButton = forwardRef<
  HTMLButtonElement,
  StartButtonProps
>(({ label = "Start", ...props }, ref) => {
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
      <span>{label}</span>
    </Button>
  );
});

function StopButton({ onClick, disabled, label = "Stop" }: ButtonProps) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
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
      <span>{label}</span>
    </button>
  );
}
