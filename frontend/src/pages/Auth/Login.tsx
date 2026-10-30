import React, { FormEvent, useRef, useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

import logo from "@/assets/images/first-encoding_logo.webp";
import { LoginUserOnline, SelectBubbleAttendanceForLogin } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import SpinnerLoader from "@/components/custom/SpinnerLoader";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import { Eye, EyeOff } from "lucide-react";

const incorrectCredentialsMessage = "Incorrect email or password.";
const genericLoginFailureMessage = "Unable to sign in. Please try again.";

function onlineLoginErrorMessage(error: unknown): string {
  if (typeof error === "string") {
    return /\b(incorrect|invalid)\b.*\b(email|password|credentials?)\b|\b(email|password|credentials?)\b.*\b(incorrect|invalid)\b/i.test(
      error
    )
      ? incorrectCredentialsMessage
      : genericLoginFailureMessage;
  }

  if (error instanceof Error) {
    return onlineLoginErrorMessage(error.message);
  }

  if (typeof error !== "object" || error === null) {
    return genericLoginFailureMessage;
  }

  const payload = error as Record<string, unknown>;
  const statusCode = payload.statusCode ?? payload.StatusCode;
  if (statusCode === 400 || statusCode === 401) {
    return incorrectCredentialsMessage;
  }

  const message = payload.message ?? payload.Message;
  return typeof message === "string"
    ? onlineLoginErrorMessage(message)
    : genericLoginFailureMessage;
}

const Login = () => {
  const navigate = useNavigate();
  const { online, setSynced, setAttendance } = useAuthStore();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorMsg, setErrorMsg] = useState("");
  const [loading, setLoading] = useState(false);
  const cancelRef = useRef(false);
  const [isCancellable, showCancel] = useState(true);
  const [loadingMsg, setLoadingMsg] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [attendanceCandidates, setAttendanceCandidates] = useState<any[]>([]);

  async function finishLogin(result: any) {
    const attendance = result?.Attendance;
    if (!attendance) throw new Error("Login did not return an attendance session.");
    useAuthStore.getState().setUser({ uid: attendance.UserID, email, offline: !online });
    if (useAuthStore.getState().user) {
      localStorage.setItem("user", JSON.stringify(useAuthStore.getState().user));
      setSynced(true);
      setAttendance(attendance);
      navigate("/home");
    }
  }

  async function loginUser(event: FormEvent) {
    event.preventDefault();
    setLoadingMsg("Authenticating...");
    setLoading(true);
    showCancel(true);
    cancelRef.current = false;
    setErrorMsg("");

    try {
      if (!online) console.log("no internet connection!");
      console.log("Logging in");
      const result: any = await LoginUserOnline(email, password);
      console.log("Auth Response: ", result);

      if (cancelRef.current) {
        setErrorMsg("Login cancelled.");
        setLoading(false);
        return;
      }

      if (result?.Status === "attendance_selection_required") {
        setAttendanceCandidates(result.Candidates || []);
        return;
      }
      setLoadingMsg("Syncing account…");
      showCancel(false);
      await finishLogin(result);
    } catch (error: unknown) {
      console.error(error);
      setErrorMsg(onlineLoginErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }

  async function selectAttendance(remoteID: string) {
    setLoading(true);
    setErrorMsg("");
    try {
      const result: any = await SelectBubbleAttendanceForLogin(remoteID);
      setAttendanceCandidates([]);
      await finishLogin(result);
    } catch (error: unknown) {
      setErrorMsg(onlineLoginErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }

  function cancelLogin() {
    cancelRef.current = true;
    setLoading(false);
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4 sm:px-6 bg-neutral-100 dark:bg-neutral-950">
      <Card className="relative w-full max-w-md rounded-2xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 shadow-xl">
        {/* Loading overlay */}
        {loading && (
          <div className="absolute inset-0 z-10 rounded-2xl bg-white/75 dark:bg-neutral-950/75 backdrop-blur-sm flex flex-col items-center justify-center">
            <SpinnerLoader />
            <span className="mt-3 text-sm text-neutral-800 dark:text-neutral-100 select-none">
              {loadingMsg}
            </span>
            {isCancellable && (
              <button
                type="button"
                onClick={cancelLogin}
                className="mt-4 text-xs font-medium text-red-500 hover:text-red-600 dark:hover:text-red-400 cursor-pointer"
              >
                Cancel
              </button>
            )}
          </div>
        )}

        <CardHeader className="space-y-4 flex flex-col items-center pt-6 pb-4">
          <img
            src={logo}
            alt="First Encoding Logo"
            className="h-12 w-auto sm:h-14"
          />
          <div className="space-y-1.5 text-center">
            <CardTitle className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
              Sign in to your account
            </CardTitle>
            <CardDescription className="text-sm text-neutral-600 dark:text-neutral-400">
              Use your First Encoding email and password to continue.
            </CardDescription>
          </div>
        </CardHeader>

        <CardContent className="pb-3">
          {attendanceCandidates.length > 0 && (
            <div className="mb-4 space-y-2 rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm dark:border-amber-700 dark:bg-amber-950/30">
              <p className="font-medium">Choose the Bubble shift to resume</p>
              <p className="text-xs text-muted-foreground">More than one open shift was found in the last 16 hours.</p>
              {attendanceCandidates.map((candidate) => (
                <Button key={candidate.RemoteID} type="button" variant="outline" className="h-auto w-full justify-start py-2 text-left" onClick={() => selectAttendance(candidate.RemoteID)}>
                  <span>{new Date(candidate.TimeIn).toLocaleString()}</span>
                  {candidate.ActiveTaskLabel && <span className="ml-2 text-xs text-muted-foreground">• {candidate.ActiveTaskLabel}</span>}
                </Button>
              ))}
            </div>
          )}
          <form onSubmit={loginUser} className="space-y-5">
            <div className="space-y-4">
              {/* Email */}
              <div className="grid gap-2">
                <Label
                  htmlFor="email"
                  className="text-sx font-medium text-neutral-800 dark:text-neutral-200"
                >
                  Email
                </Label>
                <Input
                  id="email"
                  type="text"
                  placeholder="name.firstencoding@gmail.com"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="
                    rounded-lg
                    bg-white dark:bg-neutral-900 text-sm
                    text-neutral-900 dark:text-neutral-50
                    placeholder:text-neutral-400 dark:placeholder:text-neutral-500
                    border border-neutral-300 dark:border-neutral-700
                    focus-visible:outline-none
                    focus-visible:ring-2 focus-visible:ring-emerald-500
                    focus-visible:border-emerald-500
                  "
                />
              </div>

              {/* Password */}
              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <Label
                    htmlFor="password"
                    className="text-sm font-medium text-neutral-800 dark:text-neutral-200"
                  >
                    Password
                  </Label>
                  {/* <button
                    type="button"
                    tabIndex={-1}
                    className="text-xs font-medium text-emerald-600 dark:text-emerald-400 hover:underline"
                  >
                    Forgot password?
                  </button> */}
                </div>

                <div className="relative">
                  <Input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    placeholder="••••••••"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="
      rounded-lg
      bg-white dark:bg-neutral-900 text-sm
      text-neutral-900 dark:text-neutral-50
      placeholder:text-neutral-400 dark:placeholder:text-neutral-500
      border border-neutral-300 dark:border-neutral-700
      focus-visible:outline-none
      focus-visible:ring-2 focus-visible:ring-emerald-500
      focus-visible:border-emerald-500
      pr-10
      appearance-none
    "
                  />

                  <button
                    type="button"
                    onClick={() => setShowPassword((prev) => !prev)}
                    className="
      absolute inset-y-0 right-0 flex items-center pr-3
      text-neutral-500 dark:text-neutral-300
      hover:text-neutral-700 dark:hover:text-neutral-100
      cursor-pointer
    "
                    aria-label={
                      showPassword ? "Hide password" : "Show password"
                    }
                  >
                    {showPassword ? (
                      <EyeOff className="w-4 h-4" />
                    ) : (
                      <Eye className="w-4 h-4" />
                    )}
                  </button>
                </div>
              </div>

              {/* Submit */}
              <Button
                type="submit"
                disabled={loading}
                className="
                  w-full mt-1
                  rounded-lg
                  bg-emerald-600 hover:bg-emerald-700
                  text-white text-sm font-semibold
                  py-2.5
                  tracking-wide
                  cursor-pointer
                  disabled:opacity-70 disabled:cursor-not-allowed
                "
              >
                {loading ? "Signing in..." : "Sign in"}
              </Button>
            </div>

            {errorMsg && (
              <p className="text-xs sm:text-sm text-red-600 dark:text-red-400 text-center mt-1">
                {errorMsg}
              </p>
            )}
          </form>
        </CardContent>

        <CardFooter className="flex flex-col gap-2.5 pb-5 pt-0">
          <p className="text-xs sm:text-sm text-neutral-600 dark:text-neutral-400 text-center">
            Having slow internet?{" "}
            <Link
              to="/offline-login"
              className="font-medium text-emerald-600 dark:text-emerald-400 hover:underline"
            >
              Sign in locally
            </Link>
            .
          </p>
          <AppFooterInfo className="mt-6" />
        </CardFooter>
      </Card>
    </div>
  );
};

export default Login;
