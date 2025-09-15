import MainLayout from "@/layouts/MainLayout";
import React from "react";
import { useNavigate } from "react-router-dom";

const Task = () => {
  const navigate = useNavigate();

  return (
    <MainLayout>
      <button onClick={() => navigate("/")}></button>
      <div></div>
    </MainLayout>
  );
};

export default Task;
