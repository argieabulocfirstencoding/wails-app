import { useState } from "react";
import { Link } from "react-router-dom";
import { Settings } from "lucide-react";
import Drawer from "@/components/custom/Drawer";

const ProfileBar = () => {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex px-6 py-4 border-t slight-shadow w-full justify-between sticky bottom-0 bg-white items-center">
      <img
        src="https://lh3.googleusercontent.com/a/ACg8ocJWYobBkgAVfMMKK1NZZoAlx3Zk9xr1ONbhYI7bJL-W6cpYSNE=s288-c-no"
        className="rounded-full border-2 border-green-500 select-none"
        style={{ height: 52, width: 52 }}
      />
      <div className="relative flex items-center group">
        <button
          onClick={() => setOpen(true)}
          className="rounded-full border-2 border-green-500 p-3 hover:bg-green-50 transition cursor-pointer"
        >
          <Settings className="w-6 h-6 text-green-600 transition-transform duration-500 group-hover:rotate-180" />
        </button>

        <span className="absolute bottom-14 left-1/2 -translate-x-1/2 bg-black text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition">
          Settings
        </span>
      </div>
      <Drawer open={open} onClose={() => setOpen(false)}>
        <div className="flex items-center justify-between mb-6 border-b pb-3">
          <h2 className="text-lg font-bold">Settings</h2>
          <button
            onClick={() => setOpen(false)}
            className="text-gray-500 hover:text-gray-700 cursor-pointer"
          >
            ✕
          </button>
        </div>

        <div className="flex flex-col gap-4">
          <Link to="/account-settings">
            <button className="w-full p-3 rounded-lg border hover:bg-green-50 transition cursor-pointer text-center">
              Account Settings
            </button>
          </Link>

          <Link to="/notifications">
            <button className="w-full p-3 rounded-lg border hover:bg-green-50 transition cursor-pointer text-center">
              Notifications
            </button>
          </Link>

          <Link to="/activity-logs">
            <button className="w-full p-3 rounded-lg border hover:bg-green-50 transition cursor-pointer text-center">
              Activity Logs
            </button>
          </Link>

          <Link to="/logout">
            <button className="w-full p-3 rounded-lg border hover:bg-red-50 transition cursor-pointer text-center text-red-600">
              Logout
            </button>
          </Link>
        </div>
      </Drawer>
    </div>
  );
};

export default ProfileBar;
