import React, { FormEvent, useState } from "react";
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
import { LoginUser } from "@/../wailsjs/go/app/App";

const Login = () => {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  async function loginUser(event: FormEvent) {
    event.preventDefault();
    try {
      const user = await LoginUser(email, password);

      if (user) {
        localStorage.setItem("user", user.Name);
        console.log(user);
        navigate("/");
      }
    } catch (error) {
      console.error(error);
    }
    // navigate("/");
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-6">
      <Card className="w-full max-w-md shadow-xl rounded-2xl border border-gray-200">
        <CardHeader className="space-y-4 flex flex-col items-center">
          <img src={logo} alt="Company Logo" className="h-14 w-auto" />
          <div className="space-y-2 text-center">
            <CardTitle className="text-2xl font-bold text-gray-800">
              Sign In to First Encoding Account
            </CardTitle>
            <CardDescription className="text-gray-500">
              Use your email and password to access your account.
            </CardDescription>
          </div>
        </CardHeader>

        <CardContent>
          <form onSubmit={loginUser} className="space-y-5">
            <div className="grid gap-2">
              <Label htmlFor="email" className="text-gray-700">
                Email
              </Label>
              <Input
                id="email"
                type="email"
                placeholder="name.firstencoding@gmail.com"
                required
                className="rounded-lg"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>

            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="password" className="text-gray-700">
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
                className="rounded-lg"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>

            <Button
              type="submit"
              className="w-full text-white font-medium rounded-lg py-2 hover:opacity-90 uppercase cursor-pointer tracking-widest"
              style={{ backgroundColor: "#166400" }}
            >
              Login
            </Button>
          </form>
        </CardContent>

        <CardFooter className="flex flex-col gap-3">
          <p className="text-sm text-gray-600 text-center">
            Don’t have an account?{" "}
            <Link
              to="/signup"
              className="text-green-600 hover:underline font-medium"
            >
              Sign Up
            </Link>
          </p>
        </CardFooter>
      </Card>
    </div>
  );
};

export default Login;
