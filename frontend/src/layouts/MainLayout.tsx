import React from "react";

interface MainLayoutProps {
  children: React.ReactNode;
}

const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
  return (
    <>
      <div className="background"></div>
      <div className="main-layout">{children}</div>
    </>
  );
};

export default MainLayout;
