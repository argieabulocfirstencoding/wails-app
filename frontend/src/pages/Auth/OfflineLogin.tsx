import React, { FormEvent, useEffect, useState } from "react";
import MainLayout from "../../layouts/MainLayout";
import { useNavigate } from "react-router-dom";
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
import { Link } from "react-router-dom";
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

  async function loginUser(event: FormEvent) {
    event.preventDefault();
    try {
      const authUser = await LoginUser(email);

      useAuthStore.getState().setUser({
        uid: authUser.localId,
        email: authUser.email,
        offline: true,
      });

      const user = useAuthStore.getState().user;
      if (user) {
        localStorage.setItem("user", JSON.stringify(user));
        console.log(user);
        navigate("/home");
      }
    } catch (error: any) {
      console.error(error);
      setErrorMsg(error.message || "Login failed");
    }
    // navigate("/");
  }

  async function fetchAccounts() {
    try {
      const accounts = await GetAccounts();

      if (accounts) {
        const emails = accounts.map((value, index) => {
          return value.Email;
        });

        setEmails(emails);
      }
    } catch (error) {
      console.log("Error fetching accounts: ", error);
    }
  }

  useEffect(() => {
    fetchAccounts();
  }, []);

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-neutral-950 px-6">
      <Card className="w-full max-w-md shadow-xl rounded-2xl border border-gray-200 dark:border-gray-800 bg-white dark:bg-neutral-900">
        <CardHeader className="space-y-4 flex flex-col items-center">
          <img src={logo} alt="Company Logo" className="h-14 w-auto" />
          <div className="space-y-2 text-center">
            <CardTitle className="text-2xl font-bold text-gray-800 dark:text-gray-100">
              Local Sign In to First Encoding Account
            </CardTitle>
            <CardDescription className="text-gray-500 dark:text-gray-400">
              You must still authenticate your account to sync your work to our
              cloud server within your attendance session.
            </CardDescription>
          </div>
        </CardHeader>

        <CardContent>
          <form onSubmit={loginUser} className="space-y-5">
            <div className="grid gap-2">
              <Label
                htmlFor="email"
                className="text-gray-700 dark:text-gray-300"
              >
                Email
              </Label>
              <Select onValueChange={setEmail} value={email}>
                <SelectTrigger
                  title={email || "Select your account"}
                  className="w-full bg-white dark:bg-neutral-800 dark:text-gray-100 border border-gray-200 dark:border-gray-700 rounded-md overflow-hidden"
                >
                  {/* truncate long selected email in trigger */}
                  <SelectValue
                    placeholder="Select your account"
                    className="truncate block"
                  />
                </SelectTrigger>

                <SelectContent className="bg-white dark:bg-neutral-800 text-gray-800 dark:text-gray-100 max-w-full">
                  {emails.map((value, index) => (
                    /* allow long emails to wrap inside dropdown items */
                    <SelectItem
                      value={value}
                      key={index}
                      className="break-words"
                    >
                      {value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Button
              type="submit"
              className="w-full text-white font-medium rounded-lg py-3 transition-colors duration-150 uppercase tracking-widest bg-green-600 hover:bg-green-700 cursor-pointer"
            >
              Local Login
            </Button>
            {errorMsg && (
              <p className="text-red-600 dark:text-red-400 text-sm text-center">
                {errorMsg}
              </p>
            )}
          </form>
        </CardContent>

        <CardFooter className="flex flex-col gap-3">
          <p className="text-sm text-gray-600 dark:text-gray-400 text-center">
            Internet is stable?{" "}
            <Link
              to="/login"
              className="text-green-600 dark:text-green-400 hover:underline font-medium"
            >
              Signin Online.
            </Link>
          </p>
        </CardFooter>
      </Card>
    </div>
  );
};

export default OfflineLogin;
