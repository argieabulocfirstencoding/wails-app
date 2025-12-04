import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Settings } from "lucide-react";
import Drawer from "@/components/custom/Drawer";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import ThemeSwitcher from "@/components/custom/ThemeSwitcher";
import { useAuthStore } from "@/store/AuthContext";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  BackendFirebaseSync,
  SyncUnsyncedRecords,
} from "@/../wailsjs/go/app/App";
import { signInWithEmailAndPassword } from "firebase/auth";
import { auth } from "@/firebase";
import SpinnerLoader from "./SpinnerLoader";

interface SyncUser {
  email: string | null | undefined;
  password: string | null | undefined;
}

const ProfileBar = () => {
  const [open, setOpen] = useState(false);
  const [logoutDialog, setLogoutDialog] = useState(false);
  const [syncUser, setSyncUser] = useState<SyncUser>({
    email: useAuthStore.getState().user?.email,
    password: undefined,
  });
  const [syncLoading, setSyncLoading] = useState(false);
  const [openSyncDialog, setOpenSyncDialog] = useState(false);
  const [syncErrorMsg, setSyncErrorMsg] = useState("");

  const { online, synced, setSynced } = useAuthStore();
  const navigate = useNavigate();

  const handleLogout = () => {
    localStorage.clear();
    navigate("/logout");
  };

  async function syncAccount() {
    setSyncErrorMsg("");
    setSyncLoading(true);
    setOpenSyncDialog(false);
    try {
      if (!online) {
        console.log("No internet connection!");
        return;
      }

      const authUser = await signInWithEmailAndPassword(
        auth,
        syncUser.email || "",
        syncUser.password || ""
      );
      const tokenResult = await authUser.user.getIdTokenResult();

      if (!authUser.user.email) return;

      const payload = {
        idToken: tokenResult.token,
        refreshToken: authUser.user.refreshToken,
        expiresIn: "3000",
        userId: authUser.user.uid,
        email: authUser.user.email,
      };

      await BackendFirebaseSync(
        payload.userId,
        payload.email,
        payload.idToken,
        payload.refreshToken,
        payload.expiresIn
      );

      useAuthStore.getState().setUser({
        uid: authUser.user.uid,
        email: authUser.user.email,
        offline: !online,
      });

      const user = useAuthStore.getState().user;
      if (user) {
        localStorage.setItem("user", JSON.stringify(user));
        console.log(user);

        await SyncUnsyncedRecords();

        setSynced(true);
        setSyncLoading(false);
      }
    } catch (error: any) {
      setOpenSyncDialog(true);
      setSyncLoading(false);
      console.error(error);

      switch (error.code) {
        case "auth/invalid-credential":
          setSyncErrorMsg("Invalid email or password. Please try again.");
          break;
        case "auth/invalid-email":
          setSyncErrorMsg("Please enter a valid email address.");
          break;
        case "auth/missing-password":
        case "auth/weak-password":
          setSyncErrorMsg("Please enter a valid password.");
          break;
        default:
          setSyncErrorMsg("Unexpected error: " + error.message);
          break;
      }
    }
  }

  return (
    <div className="sticky bottom-0">
      {!synced && online && (
        <>
          <Dialog open={openSyncDialog} onOpenChange={setOpenSyncDialog}>
            <form>
              <DialogTrigger asChild>
                <button className="py-2 px-4 bg-green-500 text-white rounded absolute bottom-28 right-4 cursor-pointer">
                  {syncLoading ? <SpinnerLoader /> : <span>Sync Now</span>}
                </button>
              </DialogTrigger>
              <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                  <DialogTitle>Sync Your Progress</DialogTitle>
                  <DialogDescription>
                    This will upload your latest work time to the server.
                    Anything not yet synced can be reviewed in your Sync
                    History.
                  </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4">
                  <div className="grid gap-3">
                    <Label htmlFor="email">Email</Label>
                    <Input
                      id="email"
                      type="email"
                      placeholder="example.firstencoding@gmail.com"
                      value={syncUser.email || ""}
                      onChange={(e) =>
                        setSyncUser((prev) => ({
                          ...prev,
                          email: e.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="grid gap-3">
                    <Label htmlFor="password">Password</Label>
                    <Input
                      id="password"
                      type="password"
                      placeholder="******"
                      value={syncUser.password || ""}
                      onChange={(e) =>
                        setSyncUser((prev) => ({
                          ...prev,
                          password: e.target.value,
                        }))
                      }
                    />
                  </div>
                  {syncErrorMsg && (
                    <div className="text-sm text-red-500 text-center">
                      {syncErrorMsg}
                    </div>
                  )}
                </div>
                <DialogFooter>
                  <DialogClose asChild>
                    <Button variant="outline">Cancel</Button>
                  </DialogClose>
                  <Button
                    type="submit"
                    onClick={syncAccount}
                    className="bg-green-500"
                  >
                    Sync Now
                  </Button>
                </DialogFooter>
              </DialogContent>
            </form>
          </Dialog>
          <div className="w-full bg-orange-300 text-sm flex justify-around">
            <span className="font-bold text-gray-800">Local Mode</span>
          </div>
        </>
      )}

      <div className="flex px-6 py-4 border-t slight-shadow w-full justify-between bg-white items-center dark:bg-neutral-900">
        <div className="flex items-center gap-4">
          <img
            src="https://lh3.googleusercontent.com/a/ACg8ocJWYobBkgAVfMMKK1NZZoAlx3Zk9xr1ONbhYI7bJL-W6cpYSNE=s288-c-no"
            className={`rounded-full border-2 ${
              online ? "border-green-500" : "border-neutral-500"
            } select-none`}
            style={{ height: 52, width: 52 }}
          />
          <span
            className={`relative pl-3 before:content-[''] before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:w-2 before:h-2 before:rounded-full ${
              online
                ? "before:bg-green-500 text-green-700"
                : "before:bg-neutral-500 text-neutral-600"
            }`}
          >
            {online ? "Online" : "Offline"}
          </span>
        </div>

        {/* Settings Button */}
        <div className="relative flex items-center group">
          <button
            onClick={() => setOpen(true)}
            className="rounded-full border-2 border-green-500 p-3 hover:bg-green-50 transition cursor-pointer dark:hover:bg-transparent"
          >
            <Settings className="w-6 h-6 text-green-600 transition-transform duration-500 group-hover:rotate-180" />
          </button>
          <span className="absolute bottom-14 left-1/2 -translate-x-1/2 bg-black text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition">
            Settings
          </span>
        </div>

        {/* Drawer Section */}
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
              <button className="w-full p-3 rounded-lg border dark:border-gray-700 hover:bg-green-50 dark:hover:bg-gray-700 transition cursor-pointer text-center font-semibold text-green-500">
                Account Settings
              </button>
            </Link>
            <Link to="/notifications">
              <button className="w-full p-3 rounded-lg border dark:border-gray-700 hover:bg-green-50 dark:hover:bg-gray-700 transition cursor-pointer text-center font-semibold text-green-500">
                Notifications
              </button>
            </Link>
            <Link to="/activity-logs">
              <button className="w-full p-3 rounded-lg border dark:border-gray-700 hover:bg-green-50 dark:hover:bg-gray-700 transition cursor-pointer text-center font-semibold text-green-500">
                Activity Logs
              </button>
            </Link>

            <button
              onClick={() => setLogoutDialog(true)}
              className="w-full p-3 rounded-lg border dark:border-gray-700 hover:bg-green-50 dark:hover:bg-gray-700 transition cursor-pointer text-center font-semibold text-green-500"
            >
              Logout
            </button>
          </div>

          <div className="flex flex-col gap-4 mt-6 px-2">
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-2">
              Theme
            </div>
            <ThemeSwitcher />
          </div>
        </Drawer>

        {/* Logout Dialog */}
        <Dialog open={logoutDialog} onOpenChange={setLogoutDialog}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle className="text-gray-900 dark:text-white">
                Confirm Logout
              </DialogTitle>
            </DialogHeader>
            <p className="text-sm text-gray-600 dark:text-gray-300">
              Logging out means ending your shift. Do you wish to proceed?
            </p>
            <DialogFooter className="flex gap-3 justify-end">
              <Button
                variant="outline"
                onClick={() => setLogoutDialog(false)}
                className="cursor-pointer"
              >
                Cancel
              </Button>
              <Button
                onClick={handleLogout}
                className="bg-red-600 hover:bg-red-700 text-white cursor-pointer"
              >
                Logout
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  );
};

export default ProfileBar;
