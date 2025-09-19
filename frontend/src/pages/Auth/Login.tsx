import React, { FormEvent } from "react";
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

const Login = () => {
  const navigate = useNavigate();

  function loginUser(event: FormEvent) {
    event.preventDefault();
    navigate("/");
  }

  return (
    <MainLayout>
      <div className="flex min-h-full items-center justify-center bg-gray-50 px-4">
        <Card className="w-full max-w-md shadow-xl rounded-2xl border border-gray-200">
          <CardHeader className="space-y-2">
            <CardTitle className="text-2xl font-bold text-gray-800">
              Login to your account
            </CardTitle>
            <CardDescription className="text-gray-500">
              Enter your email and password below to continue
            </CardDescription>
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
                  placeholder="john.doe@gmail.com"
                  required
                  className="rounded-lg"
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
                />
              </div>

              <Button
                type="submit"
                className="w-full bg-green-600 hover:bg-green-700 text-white font-medium rounded-lg py-2"
              >
                Login
              </Button>
            </form>
          </CardContent>

          <CardFooter className="flex flex-col gap-3">
            <p className="text-sm text-gray-600 text-center">
              Don’t have an account?{" "}
              <a
                href="/signup"
                className="text-green-600 hover:underline font-medium"
              >
                Sign Up
              </a>
            </p>
          </CardFooter>
        </Card>
      </div>
    </MainLayout>
  );
};

export default Login;
