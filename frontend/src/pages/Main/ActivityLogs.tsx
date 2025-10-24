import SettingsLayout from "@/layouts/SettingsLayout";
import React from "react";

const ActivityLogs = () => {
  return (
    <SettingsLayout>
      <div className="p-4">
        <h2 className="text-xl font-semibold">Activity Logs</h2>
        <p className="text-[14px] text-slate-500">
          Here you can manage your activity logs.
        </p>
        <div className="flex flex-col w-full mt-4">
          <div className="text-sm flex items-center gap-4">
            <span>Logged In</span>
            <hr className="flex-1" />
            <span className="text-[12px] cursor-pointer">
              10/22/2025 2:03 PM
            </span>
          </div>
          <div className="pl-4 flex flex-col">
            <div className="text-[12px] flex items-center gap-4">
              <div className="flex flex-col">
                <div className="truncate max-w-40">
                  Lorem ipsum dolor, sit amet consectetur adipisicing elit.
                  Voluptas impedit sint, quas optio explicabo quibusdam magni
                  corrupti autem reiciendis voluptatum facilis, nostrum eveniet
                  asperiores blanditiis. Eaque iusto expedita modi eos?
                </div>
                <span className="text-[10px]">(Account name)</span>
              </div>
              <hr className="flex-1" />
              <span className="text-[10px] cursor-pointer">
                10/22/2025 2:03 PM
              </span>
            </div>
          </div>
          <div className="text-sm flex items-center gap-4">
            <span>Logged Out</span>
            <hr className="flex-1" />
            <span className="text-[12px] cursor-pointer">
              10/22/2025 10:03 PM
            </span>
          </div>
        </div>
      </div>
    </SettingsLayout>
  );
};
export default ActivityLogs;
