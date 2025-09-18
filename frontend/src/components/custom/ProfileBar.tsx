import { useState } from "react";
import { Settings } from "lucide-react";
import Drawer from "@/components/custom/Drawer";

const ProfileBar = () => {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex px-8 py-4 border-t slight-shadow w-full justify-between sticky bottom-0 bg-white items-center">
      <img
        src="https://www.shutterstock.com/image-vector/user-profile-icon-vector-avatar-600nw-2558760599.jpg"
        className="rounded-full border-2 border-green-500 select-none"
        style={{ height: 52, width: 52 }}
      />
      <div className="relative flex items-center group">
        <button
          onClick={() => setOpen(true)}
          className="rounded-full border-2 border-green-500 p-3 hover:bg-green-50 transition cursor-pointer"
        >
          <Settings className="w-6 h-6 text-green-600" />
        </button>

        <span className="absolute bottom-14 left-1/2 -translate-x-1/2 bg-black text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition">
          Settings
        </span>
      </div>
      <Drawer open={open} onClose={() => setOpen(false)}>
        <h2 className="text-lg font-bold mb-4">Settings</h2>
        <div className="space-y-3">
          <button className="w-full p-3 rounded-lg border hover:bg-gray-100">
            Account Settings
          </button>
          <button className="w-full p-3 rounded-lg border hover:bg-gray-100">
            Notifications
          </button>
          <button className="w-full p-3 rounded-lg border hover:bg-gray-100">
            Activity Logs
          </button>
          <button className="w-full p-3 rounded-lg border hover:bg-gray-100">
            Logout
          </button>
        </div>
      </Drawer>
    </div>
  );
};

export default ProfileBar;
