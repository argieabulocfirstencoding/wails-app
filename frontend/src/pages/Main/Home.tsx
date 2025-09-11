import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import MainLayout from "../../layouts/MainLayout";
import { Link } from "react-router-dom";

const Home = () => {

  return (
    <MainLayout>
      <Link
        className="absolute top-12 right-5 font-bold px-4 py-2 rounded bg-red-500 text-white cursor-pointer"
        to={"/login"}
      >
        Logout
      </Link>
      <div>asa</div>
    </MainLayout>
  );
};

export default Home;
