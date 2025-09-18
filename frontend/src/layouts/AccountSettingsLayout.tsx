import React from "react";
import { Link } from "react-router-dom";

interface AccountSettingsLayoutProps {
  children: React.ReactNode;
}

const AccountSettingsLayout: React.FC<AccountSettingsLayoutProps> = ({
  children,
}) => {
  return (
    <div>
      <Link to="/">
        <button className="w-full p-3 rounded-lg border hover:bg-gray-100 cursor-pointer">
          Back
        </button>
      </Link>
      <div className="flex flex-col h-full w-screen">
        <div className="flex-1">{children}</div>
      </div>
    </div>
  );
};

export default AccountSettingsLayout;
