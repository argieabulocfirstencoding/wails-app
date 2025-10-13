import Loader from "@/components/custom/Loader";
import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";

const Landing = () => {
  const navigate = useNavigate();
  useEffect(() => {
    setTimeout(() => {
      navigate("/login");
    }, 2000);
  }, []);

  return (
    <div className="w-full h-[100vh] dark:bg-neutral-900 flex flex-col items-center justify-center">
      <div className="mb-5">
        <Loader />
      </div>
      <span className="text-black dark:text-white">Loading your FES App</span>
    </div>
  );
};

export default Landing;
