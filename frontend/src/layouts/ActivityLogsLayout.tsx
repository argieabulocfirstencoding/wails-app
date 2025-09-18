import ActivityLogs from "@/pages/Main/ActivityLogs";
import { Activity } from "lucide-react";
import React from "react";
import { Link } from "react-router-dom";

interface ActivityLogsLayoutProps {
  children: React.ReactNode;
}

const ActivityLogsLayout: React.FC<ActivityLogsLayoutProps> = ({
  children,
}) => {
  return (
    <div>
      <Link to="/">
        <button className="w-full p-3 rounded-lg border hover:bg-gray-100 cursor-pointer">
          Back
        </button>
      </Link>
      <div className="flex flex-col h-full w-screen">
        <div className="flex-1">{children}</div>
      </div>
    </div>
  );
};

export default ActivityLogsLayout;
