import React from "react";
import ProfileBar from "@/components/custom/ProfileBar";

interface MainLayoutProps {
  children: React.ReactNode;
}

const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
  return (
    <div className="flex flex-col h-screen w-screen bg-white text-gray-900 dark:bg-gray-900 dark:text-gray-100 transition-colors duration-300">
      <div className="flex-1 overflow-y-auto">{children}</div>
      <ProfileBar />
    </div>
  );
};

export default MainLayout;
