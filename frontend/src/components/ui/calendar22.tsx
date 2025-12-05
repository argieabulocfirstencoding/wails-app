"use client";

import * as React from "react";
import { ChevronDownIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

type Calendar22Props = {
  date: Date | undefined;
  setDate: (value: Date) => void;
};

export function Calendar22({ date, setDate }: Calendar22Props) {
  const [open, setOpen] = React.useState(false);

  return (
    <div className="flex flex-col">
      <Label htmlFor="date" className="px-1 text-[10px]">
        Filter Date
      </Label>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            id="date"
            className="w-32 h-8 text-xs justify-between font-normal"
          >
            {date
              ? date.toLocaleDateString("en-US", { timeZone: "Asia/Manila" })
              : "Select date"}
            <ChevronDownIcon />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto overflow-hidden p-0" align="start">
          <Calendar
            mode="single"
            selected={date}
            captionLayout="dropdown"
            onSelect={(d) => {
              setDate(d || new Date()); // NO GMT+8 conversion here
              setOpen(false);
            }}
          />
        </PopoverContent>
      </Popover>
    </div>
  );
}
