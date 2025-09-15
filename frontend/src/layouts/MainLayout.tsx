import React from "react";
import ProfileBar from "@/components/custom/ProfileBar";

interface MainLayoutProps {
  children: React.ReactNode;
}

const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
  return (
    <>
      <div className="background"></div>
      <div className="main-layout">
        <div className="flex flex-col h-full w-screen">
          <div className="flex-1">{children}</div>
          <ProfileBar />
        </div>
      </div>
    </>
  );
};

export default MainLayout;
