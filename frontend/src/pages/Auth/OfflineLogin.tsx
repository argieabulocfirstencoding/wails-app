import React, { FormEvent, useEffect, useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

import logo from "@/assets/images/first-encoding_logo.webp";
import { GetAccounts, LoginUser } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const OfflineLogin = () => {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [emails, setEmails] = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState("");
  const { setAttendance } = useAuthStore();

  async function loginUser(event: FormEvent) {
    event.preventDefault();
    setErrorMsg("");

    try {
      const authUser = await LoginUser(email);
      useAuthStore.getState().setUser({
        uid: authUser.AuthUser.localId,
        email: authUser.AuthUser.email,
        offline: true,
      });

      const user = useAuthStore.getState().user;
      if (user) {
        setAttendance(authUser.Attendance);
        localStorage.setItem("user", JSON.stringify(user));
        navigate("/home");
      }
    } catch (error: any) {
      console.error(error);
      setErrorMsg(error.message || "Local login failed. Please try again.");
    }
  }

  async function fetchAccounts() {
    try {
      const accounts = await GetAccounts();
      if (accounts) {
        const list = accounts.map((value) => value.Email);
        setEmails(list);
        if (list.length === 1) setEmail(list[0]);
      }
    } catch (error) {
      console.log("Error fetching accounts: ", error);
    }
  }

  useEffect(() => {
    fetchAccounts();
  }, []);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-neutral-100 dark:bg-neutral-950 px-4 sm:px-6">
      <Card className="w-full max-w-md rounded-2xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 shadow-xl">
        <CardHeader className="space-y-4 flex flex-col items-center pt-6 pb-4">
          <img
            src={logo}
            alt="First Encoding Logo"
            className="h-12 w-auto sm:h-14"
          />
          <div className="space-y-1.5 text-center">
            <CardTitle className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
              Sign in locally
            </CardTitle>
            <CardDescription className="text-sm text-neutral-600 dark:text-neutral-400">
              Use a saved offline account. You&apos;ll still need to sync to the
              cloud during your attendance session.
            </CardDescription>
          </div>
        </CardHeader>

        <CardContent className="px-6 pb-4 pt-2">
          <form onSubmit={loginUser} className="space-y-5">
            {/* Email Label + Select */}
            <div className="grid gap-2">
              <Label
                htmlFor="email"
                className="text-sm font-medium text-neutral-800 dark:text-neutral-200"
              >
                Email
              </Label>

              <Select onValueChange={setEmail} value={email}>
                <SelectTrigger
                  title={email || "Select your account"}
                  className="
            w-full h-11 px-3 rounded-lg text-sm
            bg-white dark:bg-neutral-900
            border border-neutral-300 dark:border-neutral-700
            text-neutral-900 dark:text-neutral-50
            focus-visible:ring-2 focus-visible:ring-emerald-500
            focus-visible:border-emerald-500
            transition-all
            overflow-hidden
          "
                >
                  <span className="truncate text-sm">
                    <SelectValue placeholder="Select your account" />
                  </span>
                </SelectTrigger>

                <SelectContent
                  className="
            bg-white dark:bg-neutral-900
            border border-neutral-200 dark:border-neutral-800
            text-neutral-900 dark:text-neutral-50
            rounded-md
          "
                >
                  {emails.map((value, index) => (
                    <SelectItem
                      key={index}
                      value={value}
                      className="break-words text-sm py-2"
                    >
                      {value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Submit Button */}
            <Button
              type="submit"
              className="
                  w-full mt-1
                  rounded-lg
                  bg-emerald-600 hover:bg-emerald-700
                  text-white text-sm font-semibold
                  py-2.5
                  tracking-wide
                  cursor-pointer
                  disabled:opacity-70 disabled:cursor-not-allowed"
            >
              Sign in locally
            </Button>

            {/* Error Message */}
            {errorMsg && (
              <p className="text-xs sm:text-sm text-red-600 dark:text-red-400 text-center">
                {errorMsg}
              </p>
            )}
          </form>
        </CardContent>

        <CardFooter className="flex flex-col gap-2 pb-4 pt-0">
          <p className="text-xs sm:text-sm text-neutral-600 dark:text-neutral-400 text-center">
            Internet is stable?{" "}
            <Link
              to="/login"
              className="font-medium text-emerald-600 dark:text-emerald-400 hover:underline"
            >
              Sign in online.
            </Link>
          </p>
          <AppFooterInfo className="mt-6" />
        </CardFooter>
      </Card>
    </div>
  );
};

export default OfflineLogin;
