import React, { FormEvent, useRef, useState } from "react";
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
import { BackendFirebaseSync } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";
import { signInWithEmailAndPassword } from "firebase/auth";
import { auth } from "@/firebase";
import SpinnerLoader from "@/components/custom/SpinnerLoader";

const Login = () => {
  const navigate = useNavigate();
  const { online, setSynced } = useAuthStore();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorMsg, setErrorMsg] = useState("");
  const [loading, setLoading] = useState(false);
  const cancelRef = useRef(false);
  const [isCancellable, showCancel] = useState(true);
  const [loadingMsg, setLoadingMsg] = useState("");

  async function loginUser(event: FormEvent) {
    event.preventDefault();
    setLoadingMsg("Authenticating...");
    setLoading(true);
    showCancel(true);
    cancelRef.current = false;

    try {
      if (!online) console.log("no internet connection!");

      let authUser = await signInWithEmailAndPassword(auth, email, password);
      const tokenResult = await authUser.user.getIdTokenResult();

      if (!authUser.user.email) return;

      const payload = {
        idToken: tokenResult.token,
        refreshToken: authUser.user.refreshToken,
        expiresIn: "3000", // assumes 50 minutes to expire
        userId: authUser.user.uid,
        email: authUser.user.email,
      };

      if (cancelRef.current) {
        setErrorMsg("Login Cancelled");
        setLoading(false);
        return;
      }

      setLoadingMsg("Syncing Account...");
      showCancel(false);
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

        setSynced(true);
        navigate("/home");
      }
    } catch (error: any) {
      console.error(error);

      switch (error.code) {
        case "auth/invalid-credential":
          setErrorMsg("Invalid email or password. Please try again.");
          break;

        case "auth/invalid-email":
          setErrorMsg("Please enter a valid email address.");
          break;

        case "auth/missing-password":
        case "auth/weak-password":
          setErrorMsg("Please enter a valid password.");
          break;

        default:
          setErrorMsg("Unexpected error");
          break;
      }
    } finally {
      setLoading(false);
    }
  }

  function cancelLogin() {
    cancelRef.current = true;
    setLoading(false);
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-6 bg-gray-50 dark:bg-neutral-950">
      <Card className="w-full max-w-md shadow-xl rounded-2xl border border-gray-200 dark:border-gray-800 bg-white dark:bg-neutral-900">
        <CardHeader className="space-y-4 flex flex-col items-center">
          <img src={logo} alt="Company Logo" className="h-14 w-auto" />
          <div className="space-y-2 text-center">
            <CardTitle className="text-2xl font-bold text-gray-800 dark:text-gray-100">
              Sign In to First Encoding Account
            </CardTitle>
            <CardDescription className="text-gray-500 dark:text-gray-400">
              Use your email and password to access your account.
            </CardDescription>
          </div>
        </CardHeader>

        <CardContent>
          <form onSubmit={loginUser} className="space-y-5">
            <div className="relative space-y-5 h-full w-full">
              {loading && (
                <div className="absolute h-full w-full z-10 backdrop-blur-[1px] rounded flex flex-col items-center justify-center">
                  <SpinnerLoader />
                  <span className="mt-2 select-none">{loadingMsg}</span>
                  {isCancellable && (
                    <span
                      className="cursor-pointer mt-5 text-red-500"
                      onClick={cancelLogin}
                    >
                      Cancel
                    </span>
                  )}
                </div>
              )}
              <div className="grid gap-2">
                <Label
                  htmlFor="email"
                  className="text-gray-700 dark:text-gray-300"
                >
                  Email
                </Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="name.firstencoding@gmail.com"
                  required
                  className="rounded-lg bg-white dark:bg-neutral-800 dark:text-gray-100"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>

              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <Label
                    htmlFor="password"
                    className="text-gray-700 dark:text-gray-300"
                  >
                    Password
                  </Label>
                  <a
                    href="#"
                    className="text-sm text-green-600 hover:underline font-medium"
                  >
                    Forgot password?
                  </a>
                </div>
                <Input
                  id="password"
                  type="password"
                  placeholder="••••••••"
                  required
                  className="rounded-lg bg-white dark:bg-neutral-800 dark:text-gray-100"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>

              <Button
                type="submit"
                className="w-full text-white font-medium rounded-lg py-2 hover:opacity-90 uppercase cursor-pointer tracking-widest bg-green-700 hover:bg-green-800"
              >
                Login
              </Button>
            </div>
            {errorMsg && (
              <p className="text-red-600 dark:text-red-400 text-sm text-center">
                {errorMsg}
              </p>
            )}
          </form>
        </CardContent>

        <CardFooter className="flex flex-col gap-3">
          <p className="text-sm text-gray-600 dark:text-gray-400 text-center">
            Having slow internet?{" "}
            <Link
              to="/offline-login"
              className="text-green-600 hover:underline font-medium"
            >
              SignIn Locally
            </Link>
          </p>
        </CardFooter>
      </Card>
    </div>
  );
};

export default Login;
