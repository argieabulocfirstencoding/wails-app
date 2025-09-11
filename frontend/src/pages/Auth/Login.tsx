import React, { FormEvent, useEffect } from "react";
import MainLayout from "../../layouts/MainLayout";
import { useNavigate } from "react-router-dom";

const Login = () => {
  const navigate = useNavigate();

  function loginUser(event: FormEvent) {
    event.preventDefault();
    navigate("/");
  }

  return (
    <MainLayout>
      <form
        onSubmit={loginUser}
        className="border shadow-2xl rounded-xl min-w-3/4 min-h-2/5 flex flex-col items-center p-5 bg-white gap-2"
      >
        <span className="font-bold text-2xl">Login</span>

        <div className="flex flex-col w-full">
          <span>Email</span>
          <input
            type="email"
            placeholder="Enter your email here"
            className="border border-gray-400 rounded p-2 px-4"
          />
        </div>
        <div className="flex flex-col w-full">
          <span>Password</span>
          <input
            type="password"
            placeholder="Enter your email here"
            className="border border-gray-400 rounded p-2 px-4"
          />
        </div>
        <div className="flex gap-5 items-center">
          <button className="px-4 py-2 bg-blue-500 rounded text-white font-bold cursor-pointer mt-5">
            Login
          </button>
        </div>
      </form>
    </MainLayout>
  );
};

export default Login;
