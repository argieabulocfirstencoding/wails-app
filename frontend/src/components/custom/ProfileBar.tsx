import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { intervalToDuration } from "date-fns";
import { Input } from "@/components/ui/input";

const ProfileBar = () => {
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
    <div className="flex px-8 py-4 border-t slight-shadow w-full justify-between sticky bottom-0 bg-white">
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
  );
};

export default ProfileBar;
